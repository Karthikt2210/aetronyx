package audit

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

const (
	privateKeyFile = "audit.ed25519"
	publicKeyFile  = "audit.ed25519.pub"
)

// LoadOrGenerate loads the Ed25519 keypair from dataDir, generating it if absent.
// Private key is stored at mode 0600, public key at mode 0644.
func LoadOrGenerate(dataDir string) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	privPath := filepath.Join(dataDir, privateKeyFile)

	if _, err := os.Stat(privPath); os.IsNotExist(err) {
		return generate(dataDir)
	}

	priv, err := loadPrivate(privPath)
	if err != nil {
		return nil, nil, fmt.Errorf("LoadOrGenerate load: %w", err)
	}
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return nil, nil, fmt.Errorf("LoadOrGenerate: unexpected public key type")
	}
	return priv, pub, nil
}

// LoadPublicKey loads only the public key from dataDir.
func LoadPublicKey(dataDir string) (ed25519.PublicKey, error) {
	pubPath := filepath.Join(dataDir, publicKeyFile)
	data, err := os.ReadFile(pubPath)
	if err != nil {
		return nil, fmt.Errorf("LoadPublicKey read: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("LoadPublicKey: no PEM block in %s", pubPath)
	}
	keyIface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("LoadPublicKey parse: %w", err)
	}
	pub, ok := keyIface.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("LoadPublicKey: unexpected key type %T", keyIface)
	}
	return pub, nil
}

// generate creates a new Ed25519 keypair and writes it to dataDir.
func generate(dataDir string) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate keypair: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("generate mkdir: %w", err)
	}

	// Marshal private key (PKCS8).
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("generate marshal private: %w", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})
	privPath := filepath.Join(dataDir, privateKeyFile)
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		return nil, nil, fmt.Errorf("generate write private: %w", err)
	}

	// Marshal public key (PKIX/SubjectPublicKeyInfo).
	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, nil, fmt.Errorf("generate marshal public: %w", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	pubPath := filepath.Join(dataDir, publicKeyFile)
	if err := os.WriteFile(pubPath, pubPEM, 0o644); err != nil {
		return nil, nil, fmt.Errorf("generate write public: %w", err)
	}

	return priv, pub, nil
}

// loadPrivate reads and decodes a PEM-encoded PKCS8 private key file.
func loadPrivate(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loadPrivate read: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("loadPrivate: no PEM block in %s", path)
	}
	keyIface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("loadPrivate parse: %w", err)
	}
	priv, ok := keyIface.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("loadPrivate: unexpected key type %T", keyIface)
	}
	return priv, nil
}
