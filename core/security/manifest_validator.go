package security

import (
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
	_, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, err
	}
	if _, err := m.canonicalize(manifestJSON); err != nil {
		return false, err
	}
	return true, nil
}
