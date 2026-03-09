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

func TestManifestValidator_MissingSignature(t *testing.T) {
	kr := NewKeyRing()
	validator := NewManifestValidator(kr)
	ok, err := validator.Verify([]byte("{}"), "")
	if ok || err == nil {
		t.Errorf("Expected error for missing signature")
	}
}

func TestManifestValidator_InvalidHexEncoding(t *testing.T) {
	kr := NewKeyRing()
	validator := NewManifestValidator(kr)
	ok, err := validator.Verify([]byte("{}"), "not-a-hex-string!")
	if ok || err == nil {
		t.Errorf("Expected error for invalid hex encoding")
	}
}

func TestManifestValidator_CanonicalJSONSerialization(t *testing.T) {
	kr := NewKeyRing()
	validator := NewManifestValidator(kr)
	
	raw := []byte(`{"z":1,"a":2}`)
	canonical, err := validator.canonicalize(raw)
	if err != nil {
		t.Errorf("Expected canonicalize to succeed")
	}
	if string(canonical) != `{"a":2,"z":1}` {
		t.Errorf("Expected strictly alphabetical JSON keys")
	}
}
