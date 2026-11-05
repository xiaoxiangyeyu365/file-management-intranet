# Self-Signed HTTPS Support Design

## Goal

Add self-signed TLS to CloudBox so WebDAV Basic Auth credentials are never transmitted in cleartext. HTTP + HTTPS coexist; `/dav` paths on HTTP redirect to HTTPS.

## Context

Campus intranet deployment. No public domain → Let's Encrypt not viable. Self-signed CA is the only zero-cost option. Windows WebDAV mapping requires the CA certificate imported into "Trusted Root Certification Authorities" to accept HTTPS connections.

## Approach

Go native `http.Server.ListenAndServeTLS()` with auto-generated self-signed certificates. Two `http.Server` goroutines share one `gin.Engine`.

## Configuration

New `tls` section in `configs/config.yaml`:

```yaml
tls:
  enabled: true
  port: 8443               # HTTPS port; 8443 avoids Linux privileged port
  cert: "./data/tls/server.crt"
  key: "./data/tls/server.key"
  ca_cert: "./data/tls/ca.crt"
  hosts:
    - "10.242.241.92"      # Server IP(s), written into certificate SAN
```

`TLSConfig` struct in `internal/config/config.go`:

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

## Certificate Generation

Package: `internal/util/tls/generate.go`

Public API:

```go
func EnsureCertificates(cfg config.TLSConfig) (certFile, keyFile string, err error)
```

Logic:

1. Check if `cert` + `key` files exist and cert is valid for > 30 days
2. If files exist and not near expiry → use them, log expiry date
3. If missing or expiring within 30 days → generate:
   - Generate CA private key (RSA 4096) + self-signed CA certificate (valid 10 years)
   - Generate server private key (RSA 2048) + CSR
   - CA signs server certificate (valid 1 year, SAN includes `hosts` + `localhost` + `127.0.0.1`)
   - Write all files to `data/tls/`:
     - `ca.key` — permissions 0600, log warning about sensitivity
     - `ca.crt` — the file clients import for trust
     - `server.key` — permissions 0600
     - `server.crt`
   - On first generation, print client import guide:
     ```
     [TLS] Self-signed certificate generated. To trust this certificate:
       - Windows: double-click ca.crt → Install Certificate → Trusted Root Certification Authorities
       - macOS: sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ca.crt
       - Linux: sudo cp ca.crt /usr/local/share/ca-certificates/cloudbox.crt && sudo update-ca-certificates
     ```
4. On subsequent starts with existing cert, log:
   ```
   [TLS] Using existing certificate (expires 2027-06-08). Import ca.crt on clients for trusted access.
   [TLS] CA private key is at data/tls/ca.key — keep it secure.
   [TLS] CA certificate SHA-256 fingerprint: A1:B2:C3:...:FF (verify before importing on clients)
   ```

## Validation

On startup, if `tls.enabled` and `tls.port == server.port`, log fatal error: `"TLS port must differ from HTTP port"`.

## Dual Server Startup

In `cmd/server/main.go`:

1. HTTP server always starts on `cfg.Server.Host:cfg.Server.Port` (existing logic, unchanged)
2. If `tls.enabled`:
   - Call `tls.EnsureCertificates(cfg.TLS)` to get cert/key paths
   - Start HTTPS server on `cfg.Server.Host:cfg.TLS.Port` in a goroutine
   - If port is occupied → log warning `"[TLS] Port 8443 is already in use, HTTPS not started"`, continue with HTTP only

Both servers share the same `*gin.Engine` instance.

## WebDAV Forced HTTPS Redirect

When `tls.enabled`, add a Gin middleware that redirects `/dav` requests on the HTTP port to HTTPS:

```go
if cfg.TLS.Enabled {
    r.Use(func(c *gin.Context) {
        if strings.HasPrefix(c.Request.URL.Path, "/dav") {
            host := c.Request.Host
            // Replace HTTP port with TLS port in Host header
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

- 301 permanent redirect — Windows WebClient follows redirects
- Only `/dav` paths are redirected; `/api/*` and web frontend remain on HTTP
- JWT tokens are in Authorization headers (not URLs), so HTTP API risk is lower

## Graceful Shutdown

On SIGINT/SIGTERM:

1. `srv.Shutdown(ctx)` — HTTP server (5s timeout)
2. `srvTLS.Shutdown(ctx)` — HTTPS server (5s timeout)
3. `auditService.Shutdown()` — flush audit buffer

## Files

| Action | Path | Purpose |
|--------|------|---------|
| Create | `internal/util/tls/generate.go` | Certificate generation, loading, expiry check |
| Modify | `internal/config/config.go` | Add `TLSConfig` struct |
| Modify | `configs/config.yaml` | Add `tls` section |
| Modify | `cmd/server/main.go` | Dual server, redirect middleware, shutdown |
| Modify | `CLAUDE.md` | HTTPS documentation |

No changes to handler, service, or repository layers — TLS is a transport concern.

## What This Does NOT Cover

- Automatic Let's Encrypt integration (requires public domain)
- Certificate revocation
- HSTS headers (self-signed cert, not applicable)
- Client certificate authentication (mTLS)
