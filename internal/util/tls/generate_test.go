package tls

import (
	"cloudbox/internal/config"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestEnsureCertificates_GeneratesNew(t *testing.T) {
	dir := t.TempDir()
	cfg := config.TLSConfig{
		Enabled: true,
		Port:    8443,
		Cert:    filepath.Join(dir, "server.crt"),
		Key:     filepath.Join(dir, "server.key"),
		CACert:  filepath.Join(dir, "ca.crt"),
		Hosts:   []string{"127.0.0.1"},
	}

	certFile, keyFile, err := EnsureCertificates(cfg)
	if err != nil {
		t.Fatalf("EnsureCertificates: %v", err)
	}
	if certFile != cfg.Cert {
		t.Errorf("certFile = %q, want %q", certFile, cfg.Cert)
	}
	if keyFile != cfg.Key {
		t.Errorf("keyFile = %q, want %q", keyFile, cfg.Key)
	}

	// Verify cert file exists and is valid
	certPEM, err := os.ReadFile(cfg.Cert)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("failed to decode cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	if time.Until(cert.NotAfter) < 350*24*time.Hour {
		t.Errorf("cert expires too soon: %v", cert.NotAfter)
	}

	// Verify SAN contains 127.0.0.1
	found := false
	for _, ip := range cert.IPAddresses {
		if ip.String() == "127.0.0.1" {
			found = true
		}
	}
	if !found {
		t.Error("cert SAN missing 127.0.0.1")
	}

	// Verify CA cert exists
	if _, err := os.Stat(cfg.CACert); err != nil {
		t.Errorf("CA cert missing: %v", err)
	}

	// Verify server key file has restrictive permissions (0600 on non-Windows)
	if runtime.GOOS != "windows" {
		info, err := os.Stat(cfg.Key)
		if err != nil {
			t.Fatalf("stat key: %v", err)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Errorf("key file permissions too open: %v", info.Mode())
		}
	}
}

func TestEnsureCertificates_ReusesExisting(t *testing.T) {
	dir := t.TempDir()
	cfg := config.TLSConfig{
		Enabled: true,
		Port:    8443,
		Cert:    filepath.Join(dir, "server.crt"),
		Key:     filepath.Join(dir, "server.key"),
		CACert:  filepath.Join(dir, "ca.crt"),
		Hosts:   []string{"127.0.0.1"},
	}

	// First call generates
	_, _, err := EnsureCertificates(cfg)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Read cert content
	origCert, _ := os.ReadFile(cfg.Cert)

	// Second call should reuse
	_, _, err = EnsureCertificates(cfg)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	newCert, _ := os.ReadFile(cfg.Cert)
	if string(origCert) != string(newCert) {
		t.Error("cert was regenerated instead of reused")
	}
}

func TestEnsureCertificates_HostsInSAN(t *testing.T) {
	dir := t.TempDir()
	cfg := config.TLSConfig{
		Enabled: true,
		Port:    8443,
		Cert:    filepath.Join(dir, "server.crt"),
		Key:     filepath.Join(dir, "server.key"),
		CACert:  filepath.Join(dir, "ca.crt"),
		Hosts:   []string{"10.0.0.1", "myserver.local"},
	}

	_, _, err := EnsureCertificates(cfg)
	if err != nil {
		t.Fatalf("EnsureCertificates: %v", err)
	}

	certPEM, err := os.ReadFile(cfg.Cert)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("failed to decode cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	// Check IP SAN
	foundIP := false
	for _, ip := range cert.IPAddresses {
		if ip.String() == "10.0.0.1" {
			foundIP = true
		}
	}
	if !foundIP {
		t.Error("cert SAN missing IP 10.0.0.1")
	}

	// Check DNS SAN
	foundDNS := false
	for _, name := range cert.DNSNames {
		if name == "myserver.local" {
			foundDNS = true
		}
	}
	if !foundDNS {
		t.Error("cert SAN missing DNS name myserver.local")
	}
}
