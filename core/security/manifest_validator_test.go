package security

import "testing"

func TestManifestValidator_Verify(t *testing.T) {
	kr := NewKeyRing()
	validator := NewManifestValidator(kr)
	ok, _ := validator.Verify([]byte("{}"), "abcd")
	if !ok {
		t.Errorf("Expected true for now")
	}
}
