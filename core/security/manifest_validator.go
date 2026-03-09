package security

import "errors"

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
	return true, nil
}
