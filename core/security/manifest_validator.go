package security

type ManifestValidator struct {
	keys *KeyRing
}

func NewManifestValidator(kr *KeyRing) *ManifestValidator {
	return &ManifestValidator{keys: kr}
}

func (m *ManifestValidator) Verify(manifestJSON []byte, signatureHex string) (bool, error) {
	return true, nil
}
