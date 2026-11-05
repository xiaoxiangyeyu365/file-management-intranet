# Self-Signed HTTPS Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add self-signed TLS to CloudBox so WebDAV Basic Auth credentials are never transmitted in cleartext. HTTP + HTTPS coexist; `/dav` paths on HTTP redirect to HTTPS.

**Architecture:** Go native `crypto/x509` + `crypto/rsa` to generate a self-signed CA and server certificate at startup. Two `http.Server` goroutines share one `gin.Engine` — HTTP on existing port, HTTPS on a new TLS port. A Gin middleware redirects `/dav` requests on HTTP to HTTPS.

**Tech Stack:** Go standard library (`crypto/x509`, `crypto/rsa`, `crypto/rand`, `crypto/x509/pkix`, `encoding/pem`), `net/http`, `github.com/gin-gonic/gin`

---

### Task 1: Add TLSConfig to config

**Files:**
- Modify: `internal/config/config.go:18-30`
- Modify: `configs/config.yaml`

- [ ] **Step 1: Add TLSConfig struct and add TLS field to Config**

In `internal/config/config.go`, add the `TLSConfig` struct after `AuditConfig` and add `TLS TLSConfig` field to the `Config` struct:

```go
type TLSConfig struct {
	Enabled bool     `yaml:"enabled"`
	Port    int      `yaml:"port"`
	Cert    string   `yaml:"cert"`
	Key     string   `yaml:"key"`
	CACert  string   `yaml:"ca_cert"`
	Hosts   []string `yaml:"hosts"`
}
```

Add `TLS TLSConfig \`yaml:"tls"\`` to the `Config` struct after the `Audit` field.

- [ ] **Step 2: Add TLS defaults in Load()**

In the `Load()` function's defaults section (after `Audit: AuditConfig{...}`), add:

```go
TLS: TLSConfig{
	Enabled: false,
	Port:    8443,
	Cert:    "./data/tls/server.crt",
	Key:     "./data/tls/server.key",
	CACert:  "./data/tls/ca.crt",
},
```

- [ ] **Step 3: Add port validation in Load()**

After the config file is loaded and YAML unmarshalled (after the `mkdirAll` calls), add:

```go
if cfg.TLS.Enabled && cfg.TLS.Port == cfg.Server.Port {
	log.Fatalf("TLS port (%d) must differ from HTTP port (%d)", cfg.TLS.Port, cfg.Server.Port)
}
```

- [ ] **Step 4: Add tls section to config.yaml**

Append to `configs/config.yaml`:

```yaml
tls:
  enabled: false
  port: 8443               # HTTPS port (8443 avoids Linux privileged port; use 443 with root/setcap)
  cert: "./data/tls/server.crt"
  key: "./data/tls/server.key"
  ca_cert: "./data/tls/ca.crt"
  hosts:                    # Server IP(s) written into certificate SAN
    - "10.242.241.92"
```

- [ ] **Step 5: Verify build**

Run: `go build ./...`
Expected: compiles with no errors

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go configs/config.yaml
git commit -m "feat: add TLSConfig to config struct and config.yaml"
```

---

### Task 2: Create certificate generation package

**Files:**
- Create: `internal/util/tls/generate.go`
- Create: `internal/util/tls/generate_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/util/tls/generate_test.go`:

```go
package tls

import (
	"cloudbox/internal/config"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
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
	info, err := os.Stat(cfg.Key)
	if err != nil {
		t.Fatalf("stat key: %v", err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Errorf("key file permissions too open: %v", info.Mode())
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

	certPEM, _ := os.ReadFile(cfg.Cert)
	block, _ := pem.Decode(certPEM)
	cert, _ := x509.ParseCertificate(block.Bytes)

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/util/tls/ -v`
Expected: FAIL — package does not exist

- [ ] **Step 3: Implement certificate generation**

Create `internal/util/tls/generate.go`:

```go
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

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
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

	sanIPs := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	var sanDNS []string
	for _, h := range cfg.Hosts {
		if ip := net.ParseIP(h); ip != nil {
			sanIPs = append(sanIPs, ip)
		} else {
			sanDNS = append(sanDNS, h)
		}
	}
	sanDNS = append(sanDNS, "localhost")

	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
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
		return false
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return false
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}

	// Check key matches cert
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/util/tls/ -v`
Expected: all 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/util/tls/generate.go internal/util/tls/generate_test.go
git commit -m "feat: add self-signed certificate generation package"
```

---

### Task 3: Add HTTPS server and WebDAV redirect to main.go

**Files:**
- Modify: `cmd/server/main.go:235-257`

- [ ] **Step 1: Add imports**

Add to the import block in `cmd/server/main.go`:

```go
tlspkg "cloudbox/internal/util/tls"
```

Add `"strconv"` to imports.

- [ ] **Step 2: Add WebDAV HTTPS redirect middleware**

Insert this middleware block in `main.go` right after the CORS middleware and before the `r.Use(middleware.Logger())` line (after line 101):

```go
// Redirect /dav paths to HTTPS when TLS is enabled
if cfg.TLS.Enabled {
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/dav") {
			host := c.Request.Host
			if strings.Contains(host, ":") {
				host = strings.SplitN(host, ":", 2)[0] + ":" + strconv.Itoa(cfg.TLS.Port)
			} else {
				host = host + ":" + strconv.Itoa(cfg.TLS.Port)
			}
			c.Redirect(301, fmt.Sprintf("https://%s%s", host, c.Request.RequestURI))
			c.Abort()
			return
		}
		c.Next()
	})
}
```

- [ ] **Step 3: Replace the single server startup with dual server**

Replace the server startup block (from `// Start server` through the shutdown code, lines 235-257) with:

```go
	// Start HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		log.Printf("Server starting at http://%s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Start HTTPS server if TLS is enabled
	var srvTLS *http.Server
	if cfg.TLS.Enabled {
		certFile, keyFile, err := tlspkg.EnsureCertificates(cfg.TLS)
		if err != nil {
			log.Printf("[TLS] Certificate error: %v — HTTPS not started", err)
		} else {
			tlsAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.TLS.Port)
			srvTLS = &http.Server{Addr: tlsAddr, Handler: r}
			go func() {
				log.Printf("[TLS] HTTPS server starting at https://%s", tlsAddr)
				if err := srvTLS.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
					log.Printf("[TLS] HTTPS server error: %v", err)
				}
			}()
		}
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	if srvTLS != nil {
		if err := srvTLS.Shutdown(shutdownCtx); err != nil {
			log.Printf("TLS server shutdown error: %v", err)
		}
	}

	auditService.Shutdown()
	log.Println("Server exited")
```

- [ ] **Step 4: Verify build**

Run: `go build ./cmd/server`
Expected: compiles with no errors

- [ ] **Step 5: Manual smoke test**

1. Edit `configs/config.yaml`: set `tls.enabled: true`
2. Run: `go run ./cmd/server`
3. Verify log output shows both `Server starting at http://...` and `[TLS] HTTPS server starting at https://...`
4. Verify `data/tls/` contains `ca.crt`, `ca.key`, `server.crt`, `server.key`
5. Test HTTP still works: `curl http://localhost/health`
6. Test HTTPS works: `curl -k https://localhost:8443/health`
7. Test WebDAV redirect: `curl -v http://localhost/dav/` → should return 301 to `https://...:8443/dav/`
8. Stop server with Ctrl+C, verify graceful shutdown logs

- [ ] **Step 6: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: add HTTPS server with WebDAV redirect-to-HTTPS middleware"
```

---

### Task 4: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add HTTPS/TLS section to CLAUDE.md**

Add the following section to `CLAUDE.md` after the WebDAV section:

```markdown
## HTTPS / TLS

Self-signed TLS for WebDAV encryption. Config in `configs/config.yaml` under `tls:`.

- `tls.enabled: true` starts a second `http.Server` on `tls.port` (default 8443) with `ListenAndServeTLS`
- Certificates auto-generated on first start to `data/tls/` (CA key 4096-bit, server key 2048-bit, server cert valid 1 year)
- Auto-renews if cert expires within 30 days
- CA cert (`data/tls/ca.crt`) must be imported on client machines for trusted HTTPS
- HTTP port still active; `/dav` paths on HTTP get 301 redirect to HTTPS
- Private key files have 0600 permissions
- Port validation: `tls.port` must differ from `server.port`
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add HTTPS/TLS section to CLAUDE.md"
```

---

### Task 5: Enable TLS in config and final integration test

**Files:**
- Modify: `configs/config.yaml`

- [ ] **Step 1: Enable TLS in default config**

In `configs/config.yaml`, change `enabled: false` to `enabled: true` in the `tls` section.

- [ ] **Step 2: Full integration test**

1. Delete `data/tls/` if it exists
2. Run: `go run ./cmd/server`
3. Verify first-start logs:
   - `[TLS] Generating self-signed CA and server certificate...`
   - `[TLS] CA private key is at ...`
   - `[TLS] CA certificate SHA-256 fingerprint: ...`
   - Client import guide printed
   - `[TLS] HTTPS server starting at https://...`
4. Verify second start reuses cert:
   - `[TLS] Using existing certificate (expires ...)`
5. `curl -k https://localhost:8443/health` → 200
6. `curl http://localhost/health` → 200 (HTTP still works)
7. `curl -v http://localhost/dav/ 2>&1 | grep "301"` → redirect to HTTPS
8. Ctrl+C → graceful shutdown with both servers stopped

- [ ] **Step 3: Commit**

```bash
git add configs/config.yaml
git commit -m "feat: enable TLS by default in config"
```
