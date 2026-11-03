package gateways

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRootCAPool verifies that the embedded trust store can validate the certificate
// chains presented by every TicketBAI endpoint. The chains under testdata/chains were
// captured with `openssl s_client -showcerts` on 2026-10-05. At that time, the
// pre-production endpoints already served the new Sectigo chain, while the production
// endpoints still served the legacy Izenpe.com one. See the ca package for details.
func TestRootCAPool(t *testing.T) {
	roots, err := rootCAPool()
	require.NoError(t, err)

	// Leaf certificates expire, so verify at the time the chains were captured.
	capturedAt := time.Date(2026, time.October, 5, 12, 0, 0, 0, time.UTC)

	chains, err := filepath.Glob(filepath.Join("testdata", "chains", "*.pem"))
	require.NoError(t, err)
	require.NotEmpty(t, chains)

	for _, path := range chains {
		host := strings.TrimSuffix(filepath.Base(path), ".pem")
		t.Run(host, func(t *testing.T) {
			certs := loadChain(t, path)
			leaf, intermediates := certs[0], x509.NewCertPool()
			for _, c := range certs[1:] {
				intermediates.AddCert(c)
			}

			_, err := leaf.Verify(x509.VerifyOptions{
				DNSName:       host,
				Roots:         roots,
				Intermediates: intermediates,
				CurrentTime:   capturedAt,
			})
			assert.NoError(t, err)
		})
	}
}

// TestRootCAPoolContents ensures the trust store includes the roots required
// by the TicketBAI services and that no leaf certificate was pinned, as the
// councils explicitly advise against.
func TestRootCAPoolContents(t *testing.T) {
	subjects := make([]string, 0)
	for _, c := range loadChain(t, filepath.Join("..", "..", "ca")) {
		subjects = append(subjects, c.Subject.CommonName)
		assert.True(t, c.IsCA, "%s is not a CA", c.Subject.CommonName)
	}

	// Legacy Izenpe hierarchy, still used in production until the switch.
	assert.Contains(t, subjects, "Izenpe.com")
	// New Sectigo RSA hierarchy, required by the technical note.
	assert.Contains(t, subjects, "Sectigo Public Server Authentication Root R46")
	// Sectigo ECC hierarchy, recommended for future ECC server certificates.
	assert.Contains(t, subjects, "Sectigo Public Server Authentication Root E46")
}

// loadChain parses every PEM certificate found in a file, or in all the .pem
// files of a directory, in order.
func loadChain(t *testing.T, path string) []*x509.Certificate {
	t.Helper()

	info, err := os.Stat(path)
	require.NoError(t, err)

	files := []string{path}
	if info.IsDir() {
		files, err = filepath.Glob(filepath.Join(path, "*.pem"))
		require.NoError(t, err)
	}

	certs := make([]*x509.Certificate, 0)
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)
		for {
			var block *pem.Block
			block, data = pem.Decode(data)
			if block == nil {
				break
			}
			c, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err)
			certs = append(certs, c)
		}
	}
	require.NotEmpty(t, certs, "no certificates in %s", path)
	return certs
}
