package tls

import (
	"cloudbox/internal/config"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// EnsureCertificates checks if server cert+key exist and are valid for >30 days.
// If missing or near expiry, generates a self-signed CA and server certificate.
// Returns cert and key file paths.
func EnsureCertificates(cfg config.TLSConfig) (certFile, keyFile string, err error) {
	certFile = cfg.Cert
	keyFile = cfg.Key

	// Check if existing cert is valid
	if existingCertValid(certFile, keyFile) {
		return certFile, keyFile, nil
	}

	// Need to generate
	log.Println("[TLS] Generating self-signed CA and server certificate...")

	if err := os.MkdirAll(filepath.Dir(certFile), 0755); err != nil {
		return "", "", fmt.Errorf("create tls dir: %w", err)
	}

	// Generate CA
	caKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", "", fmt.Errorf("generate CA key: %w", err)
	}

	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	caTemplate := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: "CloudBox CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return "", "", fmt.Errorf("create CA cert: %w", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return "", "", fmt.Errorf("parse CA cert: %w", err)
	}

	// Write CA key
	caKeyPath := filepath.Join(filepath.Dir(certFile), "ca.key")
	if err := writeKeyFile(caKeyPath, caKey); err != nil {
		return "", "", fmt.Errorf("write CA key: %w", err)
	}
	log.Printf("[TLS] CA private key is at %s — keep it secure.", caKeyPath)

	// Write CA cert
	if err := writeCertFile(cfg.CACert, caCertDER); err != nil {
		return "", "", fmt.Errorf("write CA cert: %w", err)
	}

	// Generate server cert
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("generate server key: %w", err)
	}

	sanIPSet := map[string]bool{"127.0.0.1": true, "::1": true}
	sanIPs := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	sanDNSSet := map[string]bool{"localhost": true}
	var sanDNS []string
	for _, h := range cfg.Hosts {
		if ip := net.ParseIP(h); ip != nil {
			if !sanIPSet[h] {
				sanIPSet[h] = true
				sanIPs = append(sanIPs, ip)
			}
		} else {
			if !sanDNSSet[h] {
				sanDNSSet[h] = true
				sanDNS = append(sanDNS, h)
			}
		}
	}
	sanDNS = append(sanDNS, "localhost")

	serverSerial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	serverTemplate := &x509.Certificate{
		SerialNumber: serverSerial,
		Subject:      pkix.Name{CommonName: "CloudBox Server"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  sanIPs,
		DNSNames:     sanDNS,
	}

	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return "", "", fmt.Errorf("create server cert: %w", err)
	}

	// Write server key
	if err := writeKeyFile(keyFile, serverKey); err != nil {
		return "", "", fmt.Errorf("write server key: %w", err)
	}

	// Write server cert
	if err := writeCertFile(certFile, serverCertDER); err != nil {
		return "", "", fmt.Errorf("write server cert: %w", err)
	}

	// Print fingerprint
	fingerprint := sha256.Sum256(caCertDER)
	log.Printf("[TLS] CA certificate SHA-256 fingerprint: %x", fingerprint)
	log.Println("[TLS] Self-signed certificate generated. To trust this certificate:")
	log.Println("  - Windows: double-click ca.crt → Install Certificate → Trusted Root Certification Authorities")
	log.Println("  - macOS: sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ca.crt")
	log.Println("  - Linux: sudo cp ca.crt /usr/local/share/ca-certificates/cloudbox.crt && sudo update-ca-certificates")

	return certFile, keyFile, nil
}

func existingCertValid(certFile, keyFile string) bool {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[TLS] Error reading cert file: %v", err)
		}
		return false
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[TLS] Error reading key file: %v", err)
		}
		return false
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		log.Printf("[TLS] Cert file contains invalid PEM")
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		log.Printf("[TLS] Error parsing cert: %v", err)
		return false
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		log.Printf("[TLS] Key file contains invalid PEM")
		return false
	}

	// Verify private key matches certificate's public key
	privKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		log.Printf("[TLS] Error parsing key: %v", err)
		return false
	}
	if !cert.PublicKey.(*rsa.PublicKey).Equal(privKey.Public()) {
		log.Printf("[TLS] Certificate and key do not match, regenerating...")
		return false
	}

	// Check expiry — must be valid for > 30 days
	if time.Until(cert.NotAfter) < 30*24*time.Hour {
		log.Println("[TLS] Server certificate expires within 30 days, regenerating...")
		return false
	}

	log.Printf("[TLS] Using existing certificate (expires %s). Import ca.crt on clients for trusted access.", cert.NotAfter.Format("2006-01-02"))
	return true
}

func writeKeyFile(path string, key *rsa.PrivateKey) error {
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return os.WriteFile(path, keyPEM, 0600)
}

func writeCertFile(path string, certDER []byte) error {
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	return os.WriteFile(path, certPEM, 0644)
}
