package ws

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
)

var (
	// ErrInvalidPEMBlock is returned when the PEM block cannot be decoded.
	ErrInvalidPEMBlock = errors.New("auth: failed to decode PEM block")

	// ErrInvalidPrivateKey is returned when the private key cannot be parsed.
	ErrInvalidPrivateKey = errors.New("auth: failed to parse private key")

	// ErrNotRSAKey is returned when the key is not an RSA private key.
	ErrNotRSAKey = errors.New("auth: not an RSA private key")
)

// ParsePrivateKey parses a PEM-encoded RSA private key.
// Supports both PKCS1 and PKCS8 formats.
func ParsePrivateKey(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, ErrInvalidPEMBlock
	}

	// Try PKCS1 format first (BEGIN RSA PRIVATE KEY)
	if block.Type == "RSA PRIVATE KEY" {
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidPrivateKey, err)
		}
		return key, nil
	}

	// Try PKCS8 format (BEGIN PRIVATE KEY)
	if block.Type == "PRIVATE KEY" {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidPrivateKey, err)
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, ErrNotRSAKey
		}
		return rsaKey, nil
	}

	return nil, fmt.Errorf("%w: unsupported key type %s", ErrInvalidPrivateKey, block.Type)
}

// ParsePrivateKeyString parses a PEM-encoded RSA private key from a string.
func ParsePrivateKeyString(pemStr string) (*rsa.PrivateKey, error) {
	return ParsePrivateKey([]byte(pemStr))
}

// SignMessage signs a message using RSA-PSS with SHA-256.
// This is the signing method required by Kalshi's API.
func SignMessage(privateKey *rsa.PrivateKey, message string) (string, error) {
	hashed := sha256.Sum256([]byte(message))

	signature, err := rsa.SignPSS(rand.Reader, privateKey, crypto.SHA256, hashed[:], nil)
	if err != nil {
		return "", fmt.Errorf("sign message: %w", err)
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// GenerateSignature creates the authentication signature for Kalshi API.
// The message format is: timestamp + method + path
func GenerateSignature(privateKey *rsa.PrivateKey, timestamp, method, path string) (string, error) {
	message := timestamp + method + path
	return SignMessage(privateKey, message)
}

