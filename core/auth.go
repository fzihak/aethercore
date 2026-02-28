package core

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Inviolable Rule: Layer 0 strictly uses Go stdlib ONLY.

const (
	tokenFileName = "token"
	authDomain    = "auth.aethercore.dev"
)

var (
	ErrNoToken          = errors.New("no auth token found")
	ErrInvalidToken     = errors.New("invalid or corrupted token")
	ErrTokenExpired     = errors.New("token expired")
	ErrSignatureInvalid = errors.New("invalid token signature")
	ErrInvalidIssuer    = errors.New("invalid issuer")
	ErrRefreshFailed    = errors.New("refresh failed")
)

// JWTPayload represents the expected claims in the AetherCore token.
type JWTPayload struct {
	Subject   string `json:"sub"`
	Email     string `json:"email"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Version   string `json:"ver"`
	Issuer    string `json:"iss"`
}

// TokenStore handles reading and writing the JWT to the local file system.
type TokenStore struct {
	ConfigDir string
}

func NewTokenStore() (*TokenStore, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	// For AetherCore, we standardize on ~/.aether or %APPDATA%\AetherCore
	aetherDir := filepath.Join(configDir, "aether")
	if err := os.MkdirAll(aetherDir, 0o700); err != nil {
		return nil, err
	}
	return &TokenStore{ConfigDir: aetherDir}, nil
}

func (s *TokenStore) Save(token string) error {
	path := filepath.Join(s.ConfigDir, tokenFileName)
	// Write with 0600 permissions
	return os.WriteFile(path, []byte(token), 0o600)
}

func (s *TokenStore) Load() (string, error) {
	path := filepath.Join(s.ConfigDir, tokenFileName)
	// #nosec G304 -- Path is constructed securely from the OS config directory.
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNoToken
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *TokenStore) Delete() error {
	path := filepath.Join(s.ConfigDir, tokenFileName)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Verifier handles offline JWT validation using RS256.
type Verifier struct {
	publicKey *rsa.PublicKey
}

func NewVerifier(pubKey *rsa.PublicKey) *Verifier {
	return &Verifier{publicKey: pubKey}
}

// Verify decodes the JWT, checks expiry, and verifies the RS256 signature natively.
func (v *Verifier) Verify(token string) (*JWTPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var payload JWTPayload
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().Unix() > payload.ExpiresAt {
		return nil, ErrTokenExpired
	}

	if payload.Issuer != authDomain {
		return nil, ErrInvalidIssuer
	}

	if v.publicKey != nil {
		sig, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil {
			return nil, ErrInvalidToken
		}

		hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		if err := rsa.VerifyPKCS1v15(v.publicKey, crypto.SHA256, hash[:], sig); err != nil {
			return nil, ErrSignatureInvalid
		}
	}

	return &payload, nil
}

// AuthManager coordinates token retrieval, verification, and silent refresh.
type AuthManager struct {
	store    *TokenStore
	verifier *Verifier
}

func NewAuthManager(pubKey *rsa.PublicKey) (*AuthManager, error) {
	store, err := NewTokenStore()
	if err != nil {
		return nil, err
	}
	return &AuthManager{
		store:    store,
		verifier: NewVerifier(pubKey),
	}, nil
}

// Authenticate returns the valid payload, attempting a silent refresh if needed.
func (m *AuthManager) Authenticate() (*JWTPayload, error) {
	token, err := m.store.Load()
	if err != nil {
		return nil, err
	}

	payload, err := m.verifier.Verify(token)
	if err != nil {
		if errors.Is(err, ErrTokenExpired) {
			// Try silent refresh if expired, or near expiry
			return m.SilentRefresh(token)
		}
		return nil, err
	}

	// Proactive refresh if expiry is within 7 days
	timeUntilExpiry := time.Until(time.Unix(payload.ExpiresAt, 0))
	if timeUntilExpiry < 7*24*time.Hour {
		go func() {
			_, _ = m.SilentRefresh(token) // Fire and forget background refresh
		}()
	}

	return payload, nil
}

// SilentRefresh calls the auth cloud once to get a new token.
func (m *AuthManager) SilentRefresh(oldToken string) (*JWTPayload, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+authDomain+"/refresh", strings.NewReader(`{"token":"`+oldToken+`"}`))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrRefreshFailed, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if err := m.store.Save(result.Token); err != nil {
		return nil, err
	}

	return m.verifier.Verify(result.Token)
}
