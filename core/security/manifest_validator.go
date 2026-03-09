package security

import (
	"encoding/hex"
	"errors"
)

type ManifestValidator struct {
	keys *KeyRing
}

func NewManifestValidator(kr *KeyRing) *ManifestValidator {
	return &ManifestValidator{keys: kr}
}

func (m *ManifestValidator) Verify(manifestJSON []byte, signatureHex string) (bool, error) {
	if signatureHex == "" {
		return false, errors.New("missing signature")
	}
	_, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, err
	}
	return true, nil
}
