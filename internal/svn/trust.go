package svn

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// The failure bits svn records next to a trusted certificate (svn_auth.h); a saved entry covers exactly these.
const (
	certNotYetValid  = 0x01
	certExpired      = 0x02
	certNameMismatch = 0x04
	certUnknownCA    = 0x08
	certOther        = 0x40000000
)

// ServerCert is what an https server presented, with the reasons svn would refuse it.
type ServerCert struct {
	Realm       string    `json:"realm"` // https://host:443, the key svn files a trusted certificate under
	Subject     string    `json:"subject"`
	Issuer      string    `json:"issuer"`
	Names       []string  `json:"names"`
	ValidFrom   time.Time `json:"validFrom"`
	ValidUntil  time.Time `json:"validUntil"`
	Fingerprint string    `json:"fingerprint"` // SHA-1, the way svn and VisualSVN print it
	Problems    []string  `json:"problems"`

	failures int
	der      []byte
}

func certRealm(address string) (string, string, error) {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return "", "", fmt.Errorf("%q is not an https address: %w", address, ErrInvalidRequest)
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	hostport := net.JoinHostPort(u.Hostname(), port)
	return "https://" + hostport, hostport, nil
}

// inspectCert checks the chain the way svn does and keeps the failures as svn's bits, so a saved trust matches.
func inspectCert(realm, host string, chain []*x509.Certificate, now time.Time) ServerCert {
	leaf := chain[0]
	sum := sha1.Sum(leaf.Raw)
	pairs := make([]string, len(sum))
	for i, b := range sum {
		pairs[i] = fmt.Sprintf("%02X", b)
	}
	c := ServerCert{
		Realm:       realm,
		Subject:     leaf.Subject.String(),
		Issuer:      leaf.Issuer.String(),
		Names:       append(append([]string{}, leaf.DNSNames...), ipStrings(leaf.IPAddresses)...),
		ValidFrom:   leaf.NotBefore,
		ValidUntil:  leaf.NotAfter,
		Fingerprint: strings.Join(pairs, ":"),
		der:         leaf.Raw,
	}

	pool := x509.NewCertPool()
	for _, ic := range chain[1:] {
		pool.AddCert(ic)
	}
	_, err := leaf.Verify(x509.VerifyOptions{Intermediates: pool, CurrentTime: now})
	var unknown x509.UnknownAuthorityError
	var invalid x509.CertificateInvalidError
	switch {
	case err == nil:
	case errors.As(err, &unknown):
		c.add(certUnknownCA, "it is not signed by an authority this computer trusts")
	case errors.As(err, &invalid) && invalid.Reason == x509.Expired:
	default:
		c.add(certOther, err.Error())
	}
	if now.Before(leaf.NotBefore) {
		c.add(certNotYetValid, "it is not valid yet")
	}
	if now.After(leaf.NotAfter) {
		c.add(certExpired, "it has expired")
	}
	if leaf.VerifyHostname(host) != nil {
		c.add(certNameMismatch, fmt.Sprintf("it was issued for %s, not %s", strings.Join(c.Names, ", "), host))
	}
	return c
}

func (c *ServerCert) add(bit int, problem string) {
	c.failures |= bit
	c.Problems = append(c.Problems, problem)
}

func ipStrings(ips []net.IP) []string {
	out := make([]string, len(ips))
	for i, ip := range ips {
		out[i] = ip.String()
	}
	return out
}

// FetchServerCert connects without checking the certificate, only to show it; nothing is sent over the connection.
func FetchServerCert(ctx context.Context, address string) (ServerCert, error) {
	realm, hostport, err := certRealm(address)
	if err != nil {
		return ServerCert{}, err
	}
	chain, err := peerChain(ctx, hostport)
	if err != nil {
		return ServerCert{}, fmt.Errorf("read certificate of %s: %w", hostport, err)
	}
	host, _, _ := net.SplitHostPort(hostport)
	return inspectCert(realm, host, chain, time.Now()), nil
}

func peerChain(ctx context.Context, hostport string) ([]*x509.Certificate, error) {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return nil, err
	}
	d := tls.Dialer{Config: &tls.Config{InsecureSkipVerify: true, ServerName: host}}
	conn, err := d.DialContext(ctx, "tcp", hostport)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	chain := conn.(*tls.Conn).ConnectionState().PeerCertificates
	if len(chain) == 0 {
		return nil, errors.New("server sent no certificate")
	}
	return chain, nil
}

// ConfigDir is svn's own runtime configuration folder, shared with TortoiseSVN on Windows.
func ConfigDir() (string, error) {
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "Subversion"), nil
		}
		return "", errors.New("APPDATA is not set")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("svn config folder: %w", err)
	}
	return filepath.Join(home, ".subversion"), nil
}

// trustPath is where svn looks for the certificate it accepted for realm: the folder is fixed and the file is named
// after the MD5 of the realm string.
func trustPath(configDir, realm string) string {
	sum := md5.Sum([]byte(realm))
	return filepath.Join(configDir, "auth", "svn.ssl.server", hex.EncodeToString(sum[:]))
}

// TrustServerCert writes the same entry svn writes when you answer (p) to "accept permanently", so every later svn
// command, svnch's or TortoiseSVN's, trusts this certificate for its current problems and no others.
func TrustServerCert(ctx context.Context, configDir string, c ServerCert) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(c.der) == 0 {
		return fmt.Errorf("no certificate to trust: %w", ErrInvalidRequest)
	}
	path := trustPath(configDir, c.Realm)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("trust %s: %w", c.Realm, err)
	}
	body := hashFile([][2]string{
		{"ascii_cert", base64.StdEncoding.EncodeToString(c.der)},
		{"failures", strconv.Itoa(c.failures)},
		{"svn:realmstring", c.Realm},
	})
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return fmt.Errorf("trust %s: %w", c.Realm, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("trust %s: %w", c.Realm, err)
	}
	return nil
}

// trusted tells whether svn's store already accepts this certificate with these problems.
func trusted(configDir string, c ServerCert) bool {
	if c.failures == 0 {
		return true
	}
	data, err := os.ReadFile(trustPath(configDir, c.Realm))
	if err != nil {
		return false
	}
	saved := readHashFile(data)
	accepted, err := strconv.Atoi(saved["failures"])
	return err == nil && saved["ascii_cert"] == base64.StdEncoding.EncodeToString(c.der) && c.failures&^accepted == 0
}

// hashFile writes svn's "K len / key / V len / value / END" format that its auth store uses.
func hashFile(kv [][2]string) []byte {
	var b bytes.Buffer
	for _, p := range kv {
		fmt.Fprintf(&b, "K %d\n%s\nV %d\n%s\n", len(p[0]), p[0], len(p[1]), p[1])
	}
	b.WriteString("END\n")
	return b.Bytes()
}

func readHashFile(data []byte) map[string]string {
	out := map[string]string{}
	rest := string(data)
	next := func(prefix string) (string, bool) {
		line, after, ok := strings.Cut(rest, "\n")
		n, err := strconv.Atoi(strings.TrimPrefix(line, prefix))
		if !ok || !strings.HasPrefix(line, prefix) || err != nil || n < 0 || n > len(after) {
			return "", false
		}
		rest = strings.TrimPrefix(after[n:], "\n")
		return after[:n], true
	}
	for !strings.HasPrefix(rest, "END") {
		k, ok := next("K ")
		if !ok {
			break
		}
		v, ok := next("V ")
		if !ok {
			break
		}
		out[k] = v
	}
	return out
}

// TrustingTransport checks certificates like svn does, so the repository index read over plain https accepts what
// svn accepts: the system's authorities, or a certificate the user trusted into svn's store.
func TrustingTransport(configDir string) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialTLSContext = func(ctx context.Context, network, hostport string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(hostport)
		if err != nil {
			return nil, err
		}
		d := tls.Dialer{Config: &tls.Config{InsecureSkipVerify: true, ServerName: host}}
		conn, err := d.DialContext(ctx, network, hostport)
		if err != nil {
			return nil, err
		}
		chain := conn.(*tls.Conn).ConnectionState().PeerCertificates
		if len(chain) == 0 {
			conn.Close()
			return nil, fmt.Errorf("%s sent no certificate: %w", hostport, ErrCertUntrusted)
		}
		c := inspectCert("https://"+hostport, host, chain, time.Now())
		if !trusted(configDir, c) {
			conn.Close()
			return nil, fmt.Errorf("%s: %s: %w", hostport, strings.Join(c.Problems, "; "), ErrCertUntrusted)
		}
		return conn, nil
	}
	return t
}
