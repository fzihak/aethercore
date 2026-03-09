package security

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
)

type ManifestValidator struct {
	keys *KeyRing
}

func NewManifestValidator(kr *KeyRing) *ManifestValidator {
	return &ManifestValidator{keys: kr}
}

func (m *ManifestValidator) canonicalize(raw []byte) ([]byte, error) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	return json.Marshal(parsed)
}

func (m *ManifestValidator) Verify(manifestJSON []byte, signatureHex string) (bool, error) {
	if signatureHex == "" {
		return false, errors.New("missing signature")
	}
	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, err
	}
	if len(sigBytes) != ed25519.SignatureSize {
		return false, errors.New("invalid signature length")
	}
	
	canonicalMsg, err := m.canonicalize(manifestJSON)
	if err != nil {
		return false, err
	}

	trustedKeys := m.keys.Keys()
	if len(trustedKeys) == 0 {
		return false, errors.New("no trusted public keys loaded")
	}

	for _, pubKey := range trustedKeys {
		if ed25519.Verify(pubKey, canonicalMsg, sigBytes) {
			return true, nil
		}
	}

	return false, errors.New("signature verification failed against all trusted keys")
}
