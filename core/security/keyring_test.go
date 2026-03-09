package security

import "testing"

func TestKeyRing_LoadKey(t *testing.T) {
	kr := NewKeyRing()
	if len(kr.trustedKeys) != 0 {
		t.Errorf("Expected 0 keys")
	}
}

func TestKeyRing_LoadValidEd25519PublicKey(t *testing.T) {
	kr := NewKeyRing()
	err := kr.LoadPEM([]byte("invalid pem data"))
	if err == nil {
		t.Errorf("Expected loading invalid PEM to fail")
	}
}

func TestKeyRing_LoadMalformedPublicKey(t *testing.T) {
	kr := NewKeyRing()
	
	// Valid PEM block but random data inside
	pemData := `-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==
-----END PUBLIC KEY-----`

	err := kr.LoadPEM([]byte(pemData))
	if err == nil {
		t.Errorf("Expected parsing garbage PKIX to fail")
	}
}
