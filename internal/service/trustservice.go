package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Cn8001/svnch/internal/svn"
)

// TrustService accepts a server's https certificate into svn's own store, the in-app version of answering (p).
type TrustService struct {
	configDir string // svn's config folder; empty when it could not be found, and then nothing can be trusted
}

func NewTrustService(configDir string) *TrustService {
	return &TrustService{configDir: configDir}
}

func (s *TrustService) Cert(ctx context.Context, url string) (svn.ServerCert, error) {
	ctx, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	return svn.FetchServerCert(ctx, url)
}

// Trust takes the fingerprint the user was shown and fetches the certificate again, so what gets trusted is exactly
// what was on screen and not whatever the server sends by the time the button is pressed.
func (s *TrustService) Trust(ctx context.Context, url, fingerprint string) error {
	if s.configDir == "" {
		return fmt.Errorf("svn config folder unknown: %w", svn.ErrInvalidRequest)
	}
	c, err := s.Cert(ctx, url)
	if err != nil {
		return err
	}
	if c.Fingerprint != fingerprint {
		return fmt.Errorf("the certificate of %s changed since it was shown: %w", c.Realm, svn.ErrInvalidRequest)
	}
	if err := svn.TrustServerCert(ctx, s.configDir, c); err != nil {
		return fmt.Errorf("trust certificate: %w", err)
	}
	return nil
}
