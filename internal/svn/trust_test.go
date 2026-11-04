package svn

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"slices"
	"testing"
	"time"
)

func selfSigned(t *testing.T, name string, from, until time.Time) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: name},
		DNSNames:     []string{name},
		NotBefore:    from,
		NotAfter:     until,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func TestInspectCert_SelfSignedByIpIsUnknownCaAndNameMismatch(t *testing.T) {
	// Arrange
	now := time.Now()
	cert := selfSigned(t, "svnhost", now.Add(-time.Hour), now.Add(time.Hour))

	// Act
	c := inspectCert("https://192.168.1.124:443", "192.168.1.124", []*x509.Certificate{cert}, now)

	// Assert
	if c.failures != certUnknownCA|certNameMismatch {
		t.Errorf("failures = %#x, want unknown CA and name mismatch", c.failures)
	}
	if len(c.Problems) != 2 || len(c.Fingerprint) != 59 {
		t.Errorf("problems %q, fingerprint %q", c.Problems, c.Fingerprint)
	}
}

func TestTrustServerCert_StoreAcceptsThatCertOnly(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	now := time.Now()
	cert := selfSigned(t, "svnhost", now.Add(-time.Hour), now.Add(time.Hour))
	c := inspectCert("https://192.168.1.124:443", "192.168.1.124", []*x509.Certificate{cert}, now)
	other := inspectCert("https://192.168.1.124:443", "192.168.1.124", []*x509.Certificate{selfSigned(t, "svnhost", now.Add(-time.Hour), now.Add(time.Hour))}, now)
	expired := inspectCert("https://192.168.1.124:443", "192.168.1.124", []*x509.Certificate{cert}, now.Add(2*time.Hour))

	// Act
	err := TrustServerCert(context.Background(), dir, c)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !trusted(dir, c) {
		t.Error("trusted certificate not accepted")
	}
	if trusted(dir, other) {
		t.Error("a different certificate was accepted")
	}
	if trusted(dir, expired) {
		t.Error("the same certificate with a new problem (expired) was accepted")
	}
}

func TestReadHashFile_ReadsWhatHashFileWrites(t *testing.T) {
	// Arrange
	kv := [][2]string{{"ascii_cert", "QUJD"}, {"failures", "12"}, {"svn:realmstring", "https://h:443"}}

	// Act
	got := readHashFile(hashFile(kv))

	// Assert
	for _, p := range kv {
		if got[p[0]] != p[1] {
			t.Errorf("%s = %q, want %q", p[0], got[p[0]], p[1])
		}
	}
}

func TestCertRealm_AddsDefaultPort(t *testing.T) {
	// Arrange
	cases := map[string]string{
		"https://192.168.1.124/svn":    "https://192.168.1.124:443",
		"https://svn.example.org:8443": "https://svn.example.org:8443",
	}

	// Act
	var got []string
	for in := range cases {
		r, _, err := certRealm(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		got = append(got, r)
	}

	// Assert
	for _, want := range cases {
		if !slices.Contains(got, want) {
			t.Errorf("realms %q miss %q", got, want)
		}
	}
	if _, _, err := certRealm("http://192.168.1.124/svn"); err == nil {
		t.Error("http address accepted")
	}
}
