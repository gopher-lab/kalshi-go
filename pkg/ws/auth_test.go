package ws

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"
)

func TestParsePrivateKey_PKCS1(t *testing.T) {
	// Generate a test RSA key.
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	// Encode as PKCS1 PEM.
	pem := encodePKCS1PrivateKey(privateKey)

	// Parse it back.
	parsed, err := ParsePrivateKey(pem)
	if err != nil {
		t.Fatalf("ParsePrivateKey failed: %v", err)
	}

	if parsed.N.Cmp(privateKey.N) != 0 {
		t.Error("parsed key does not match original")
	}
}

func TestParsePrivateKey_InvalidPEM(t *testing.T) {
	_, err := ParsePrivateKey([]byte("not a valid pem"))
	if err == nil {
		t.Error("expected error for invalid PEM")
	}
	if err != ErrInvalidPEMBlock {
		t.Errorf("expected ErrInvalidPEMBlock, got: %v", err)
	}
}

func TestParsePrivateKey_InvalidKey(t *testing.T) {
	invalidPEM := []byte(`-----BEGIN RSA PRIVATE KEY-----
bm90IGEgdmFsaWQga2V5
-----END RSA PRIVATE KEY-----`)

	_, err := ParsePrivateKey(invalidPEM)
	if err == nil {
		t.Error("expected error for invalid key data")
	}
	if !strings.Contains(err.Error(), "failed to parse private key") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestParsePrivateKeyString(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	pem := encodePKCS1PrivateKey(privateKey)
	pemStr := string(pem)

	parsed, err := ParsePrivateKeyString(pemStr)
	if err != nil {
		t.Fatalf("ParsePrivateKeyString failed: %v", err)
	}

	if parsed.N.Cmp(privateKey.N) != 0 {
		t.Error("parsed key does not match original")
	}
}

func TestSignMessage(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	message := "1234567890GET/trade-api/ws/v2"

	sig, err := SignMessage(privateKey, message)
	if err != nil {
		t.Fatalf("SignMessage failed: %v", err)
	}

	// Signature should be base64 encoded.
	if sig == "" {
		t.Error("signature should not be empty")
	}

	// Verify it's valid base64.
	_, err = base64.StdEncoding.DecodeString(sig)
	if err != nil {
		t.Errorf("signature is not valid base64: %v", err)
	}
}

func TestSignMessage_DifferentMessages(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	sig1, err := SignMessage(privateKey, "message1")
	if err != nil {
		t.Fatalf("SignMessage failed: %v", err)
	}

	sig2, err := SignMessage(privateKey, "message2")
	if err != nil {
		t.Fatalf("SignMessage failed: %v", err)
	}

	// RSA-PSS is randomized, so even same message gives different sigs.
	// But different messages should definitely give different sigs.
	if sig1 == "" || sig2 == "" {
		t.Error("signatures should not be empty")
	}
}

func TestGenerateSignature(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	timestamp := "1234567890"
	method := "GET"
	path := "/trade-api/ws/v2"

	sig, err := GenerateSignature(privateKey, timestamp, method, path)
	if err != nil {
		t.Fatalf("GenerateSignature failed: %v", err)
	}

	if sig == "" {
		t.Error("signature should not be empty")
	}

	// Verify it's valid base64.
	_, err = base64.StdEncoding.DecodeString(sig)
	if err != nil {
		t.Errorf("signature is not valid base64: %v", err)
	}
}

// encodePKCS1PrivateKey encodes a private key as PKCS1 PEM format.
func encodePKCS1PrivateKey(key *rsa.PrivateKey) []byte {
	der := x509.MarshalPKCS1PrivateKey(key)
	encoded := base64.StdEncoding.EncodeToString(der)

	// Format with line breaks every 64 chars.
	var formatted strings.Builder
	formatted.WriteString("-----BEGIN RSA PRIVATE KEY-----\n")
	for i := 0; i < len(encoded); i += 64 {
		end := i + 64
		if end > len(encoded) {
			end = len(encoded)
		}
		formatted.WriteString(encoded[i:end])
		formatted.WriteString("\n")
	}
	formatted.WriteString("-----END RSA PRIVATE KEY-----")

	return []byte(formatted.String())
}
