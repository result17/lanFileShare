package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// KeyPair represents an RSA key pair
type KeyPair struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
}

// GenerateKeyPair generates a new RSA key pair with the specified bit size
func GenerateKeyPair(bitSize int) (*KeyPair, error) {
	if bitSize < 1024 {
		return nil, fmt.Errorf("key size must be at least 1024 bits")
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key pair: %w", err)
	}

	return &KeyPair{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
	}, nil
}

// PrivateKeyToPEM converts an RSA private key to PEM format
func PrivateKeyToPEM(privateKey *rsa.PrivateKey) ([]byte, error) {
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})
	return privateKeyPEM, nil
}

// PublicKeyToPEM converts an RSA public key to PEM format
func PublicKeyToPEM(publicKey *rsa.PublicKey) ([]byte, error) {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})
	return publicKeyPEM, nil
}

// PrivateKeyFromPEM parses an RSA private key from PEM format
func PrivateKeyFromPEM(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	if block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("invalid PEM block type: %s", block.Type)
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return privateKey, nil
}

// PublicKeyFromPEM parses an RSA public key from PEM format
func PublicKeyFromPEM(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	if block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("invalid PEM block type: %s", block.Type)
	}

	publicKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	publicKey, ok := publicKeyInterface.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not an RSA public key")
	}

	return publicKey, nil
}

// ValidateKeyPair validates that a private and public key form a valid pair
func ValidateKeyPair(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) error {
	// Check if the public key from private key matches the provided public key
	if privateKey.PublicKey.N.Cmp(publicKey.N) != 0 {
		return fmt.Errorf("public key does not match private key")
	}

	if privateKey.PublicKey.E != publicKey.E {
		return fmt.Errorf("public key exponent does not match private key")
	}

	return nil
}

// ValidatePublicKey performs basic validation on an RSA public key
func ValidatePublicKey(publicKey *rsa.PublicKey) error {
	if publicKey == nil {
		return fmt.Errorf("public key is nil")
	}

	if publicKey.N == nil {
		return fmt.Errorf("public key modulus is nil")
	}

	// Check minimum key size (1024 bits)
	if publicKey.N.BitLen() < 1024 {
		return fmt.Errorf("public key size (%d bits) is too small, minimum 1024 bits required", publicKey.N.BitLen())
	}

	// Check common exponent values
	if publicKey.E != 65537 && publicKey.E != 3 {
		return fmt.Errorf("unusual public exponent: %d", publicKey.E)
	}

	return nil
}

// ValidatePrivateKey performs basic validation on an RSA private key
func ValidatePrivateKey(privateKey *rsa.PrivateKey) error {
	if privateKey == nil {
		return fmt.Errorf("private key is nil")
	}

	// Validate the public key component
	if err := ValidatePublicKey(&privateKey.PublicKey); err != nil {
		return fmt.Errorf("invalid public key component: %w", err)
	}

	// Validate private key components
	if privateKey.D == nil {
		return fmt.Errorf("private exponent is nil")
	}

	if len(privateKey.Primes) < 2 {
		return fmt.Errorf("insufficient prime factors")
	}

	// Validate that the key is mathematically consistent
	err := privateKey.Validate()
	if err != nil {
		return fmt.Errorf("private key validation failed: %w", err)
	}

	return nil
}

// SecureCompareBytes performs constant-time comparison of byte slices
func SecureCompareBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}

	return result == 0
}
