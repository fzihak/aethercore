package core

// Inviolable Rule: Layer 0 strictly uses Go stdlib ONLY.

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

var (
	// ErrCertNotFound is returned when the certificate store doesn't exist yet.
	ErrCertNotFound = errors.New("mtls: certificate store not found")
	// ErrCAExpired is returned when the on-disk CA certificate has expired.
	ErrCAExpired = errors.New("mtls: CA certificate has expired")
)

// NodeIdentity holds the mTLS credentials for a single AetherCore node.
// The CA portion is shared across the mesh; the leaf cert is per-node.
type NodeIdentity struct {
	// CACert is the self-signed CA used to sign all node leaf certificates.
	CACert *x509.Certificate
	// CACertPEM is the PEM-encoded CA certificate for distribution to peers.
	CACertPEM []byte
	// TLSConfig is the pre-built tls.Config for gRPC server/client use.
	TLSConfig *tls.Config
}

// certDir returns the filesystem path where certs are stored.
// On Linux/macOS: ~/.aether/certs/
// On Windows:     %APPDATA%\aether\certs\.
func certDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("mtls: cannot determine config dir: %w", err)
	}
	return filepath.Join(base, "aether", "certs"), nil
}

// LoadOrCreateIdentity loads existing mTLS credentials from disk, or generates
// a fresh CA + leaf certificate pair on first boot. All crypto uses ECDSA P-256
// (stdlib-only, no external deps).
//
// Certificate lifetimes:
//   - CA:   10 years (long-lived root of trust for the mesh)
//   - Leaf: 90 days  (rotated regularly to limit blast radius)
func LoadOrCreateIdentity(nodeID string) (*NodeIdentity, error) {
	dir, err := certDir()
	if err != nil {
		return nil, err
	}

	caKeyPath := filepath.Join(dir, "ca.key")
	caCertPath := filepath.Join(dir, "ca.crt")
	leafKeyPath := filepath.Join(dir, "node.key")
	leafCertPath := filepath.Join(dir, "node.crt")

	// If any file is missing, regenerate everything from scratch.
	for _, p := range []string{caKeyPath, caCertPath, leafKeyPath, leafCertPath} {
		if _, statErr := os.Stat(p); os.IsNotExist(statErr) {
			WithComponent("mtls").Info("cert_store_missing_regenerating", "path", p)
			return generateAndSave(nodeID, dir, caKeyPath, caCertPath, leafKeyPath, leafCertPath)
		}
	}

	// Load existing CA cert to check expiry.
	caCertPEM, err := os.ReadFile(caCertPath) // #nosec G304 -- path is constructed from UserConfigDir + hardcoded suffix
	if err != nil {
		return nil, fmt.Errorf("mtls: read ca cert: %w", err)
	}
	block, _ := pem.Decode(caCertPEM)
	if block == nil {
		return nil, ErrCertNotFound
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("mtls: parse ca cert: %w", err)
	}
	if time.Now().After(caCert.NotAfter) {
		WithComponent("mtls").Warn("ca_cert_expired_regenerating")
		return generateAndSave(nodeID, dir, caKeyPath, caCertPath, leafKeyPath, leafCertPath)
	}

	// Load TLS keypair (leaf cert + key).
	tlsCert, err := tls.LoadX509KeyPair(leafCertPath, leafKeyPath)
	if err != nil {
		return nil, fmt.Errorf("mtls: load leaf keypair: %w", err)
	}

	// Build certificate pool trusting ONLY our own CA.
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCertPEM) {
		return nil, errors.New("mtls: failed to append CA cert to pool")
	}

	tlsCfg := buildTLSConfig(&tlsCert, pool)

	WithComponent("mtls").Info("identity_loaded", "node_id", nodeID, "ca_expires", caCert.NotAfter.Format(time.RFC3339))

	return &NodeIdentity{
		CACert:    caCert,
		CACertPEM: caCertPEM,
		TLSConfig: tlsCfg,
	}, nil
}

// generateAndSave creates a fresh CA + leaf certificate pair and writes them to disk.
func generateAndSave(nodeID, dir, caKeyPath, caCertPath, leafKeyPath, leafCertPath string) (*NodeIdentity, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("mtls: create cert dir: %w", err)
	}

	// 1. Generate CA key + self-signed certificate.
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("mtls: generate CA key: %w", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          newSerial(),
		Subject:               pkix.Name{CommonName: "AetherCore Mesh CA", Organization: []string{"AetherCore"}},
		NotBefore:             time.Now().Add(-time.Minute), // small backdate for clock skew
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("mtls: sign CA cert: %w", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, fmt.Errorf("mtls: parse new CA cert: %w", err)
	}

	// 2. Generate leaf node key + certificate signed by the CA.
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("mtls: generate leaf key: %w", err)
	}

	leafTemplate := &x509.Certificate{
		SerialNumber: newSerial(),
		Subject:      pkix.Name{CommonName: nodeID, Organization: []string{"AetherCore"}},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(90 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{nodeID, "localhost"},
	}

	leafCertDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("mtls: sign leaf cert: %w", err)
	}

	// 3. Encode and write all four files (0600 — owner only).
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	caKeyPEM, err := encodeECKey(caKey)
	if err != nil {
		return nil, err
	}
	leafCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafCertDER})
	leafKeyPEM, err := encodeECKey(leafKey)
	if err != nil {
		return nil, err
	}

	for path, data := range map[string][]byte{
		caKeyPath:    caKeyPEM,
		caCertPath:   caCertPEM,
		leafKeyPath:  leafKeyPEM,
		leafCertPath: leafCertPEM,
	} {
		if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil {
			return nil, fmt.Errorf("mtls: write %s: %w", path, writeErr)
		}
	}

	// 4. Build TLS config.
	tlsCert, err := tls.X509KeyPair(leafCertPEM, leafKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("mtls: build tls cert: %w", err)
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caCertPEM)

	WithComponent("mtls").Info("identity_generated",
		"node_id", nodeID,
		"ca_expires", caTemplate.NotAfter.Format(time.RFC3339),
		"leaf_expires", leafTemplate.NotAfter.Format(time.RFC3339),
	)

	return &NodeIdentity{
		CACert:    caCert,
		CACertPEM: caCertPEM,
		TLSConfig: buildTLSConfig(&tlsCert, pool),
	}, nil
}

// buildTLSConfig constructs a strict TLS 1.3-only config for mTLS mesh comms.
func buildTLSConfig(cert *tls.Certificate, pool *x509.CertPool) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{*cert},
		ClientCAs:    pool,
		RootCAs:      pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}
}

// encodeECKey marshals an ECDSA private key to PEM.
func encodeECKey(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("mtls: marshal ec key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), nil
}

// newTLSListener wraps a plain TCP listener in a TLS server acceptor.
// The provided cfg must have Certificates and ClientCAs set (server-side mTLS).
func newTLSListener(inner net.Listener, cfg *tls.Config) net.Listener {
	return tls.NewListener(inner, cfg)
}

// newTLSClientConn wraps a raw TCP connection in a TLS client handshake.
// serverName is used for SNI; it must match a SAN in the server's leaf cert.
func newTLSClientConn(conn net.Conn, cfg *tls.Config, serverName string) *tls.Conn {
	c := cfg.Clone()
	c.ServerName = serverName
	return tls.Client(conn, c)
}

// newSerial generates a cryptographically random certificate serial number.
func newSerial() *big.Int {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		// Fallback to timestamp-based serial — never zero.
		return big.NewInt(time.Now().UnixNano())
	}
	return serial
}
