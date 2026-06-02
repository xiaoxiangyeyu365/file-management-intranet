package handler

import (
	"cloudbox/internal/service"
	"context"
	"encoding/base64"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
)

func BasicAuthMiddleware(authService *service.AuthService, audit service.AuditRecorder) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Windows WebClient sends unauthenticated OPTIONS to discover WebDAV capabilities.
		// The webdav.Handler calls Stat() during handleOptions, so we need a stub userID.
		if c.Request.Method == "OPTIONS" {
			ctx := context.WithValue(c.Request.Context(), "userID", int64(0))
			ctx = context.WithValue(ctx, "clientIP", c.ClientIP())
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		authPreview := authHeader
		if len(authPreview) > 30 {
			authPreview = authPreview[:30] + "..."
		}

		if authHeader == "" || !strings.HasPrefix(authHeader, "Basic ") {
			log.Printf("[WebDAV] %s %s -> 401 (no auth) UA=%q",
				c.Request.Method, c.Request.URL.Path,
				c.GetHeader("User-Agent"))
			c.Header("WWW-Authenticate", `Basic realm="CloudBox WebDAV"`)
			c.AbortWithStatus(401)
			return
		}

		log.Printf("[WebDAV] %s %s -> auth=%q UA=%q",
			c.Request.Method, c.Request.URL.Path,
			authPreview, c.GetHeader("User-Agent"))

		decoded, err := base64.StdEncoding.DecodeString(authHeader[6:])
		if err != nil {
			c.Header("WWW-Authenticate", `Basic realm="CloudBox WebDAV"`)
			c.AbortWithStatus(401)
			return
		}

		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) != 2 {
			c.Header("WWW-Authenticate", `Basic realm="CloudBox WebDAV"`)
			c.AbortWithStatus(401)
			return
		}
		username, password := parts[0], parts[1]

		ctx := context.WithValue(c.Request.Context(), "clientIP", c.ClientIP())
		c.Request = c.Request.WithContext(ctx)

		user, err := authService.ValidateCredentials(ctx, username, password)
		if err != nil {
			log.Printf("[WebDAV] %s %s -> 401 (bad creds) user=%q",
				c.Request.Method, c.Request.URL.Path, username)
			audit.Record(ctx, "user.login_failed", "user", 0, username, "")
			c.Header("WWW-Authenticate", `Basic realm="CloudBox WebDAV"`)
			c.AbortWithStatus(401)
			return
		}

		log.Printf("[WebDAV] %s %s -> OK user=%q",
			c.Request.Method, c.Request.URL.Path, user.Username)

		c.Set("userID", user.ID)
		c.Set("username", user.Username)
		c.Set("role", user.Role)

		ctx = context.WithValue(ctx, "userID", user.ID)
		ctx = context.WithValue(ctx, "username", user.Username)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// GetUserIDFromContext extracts userID from a context.Context value.
// Used by WebDAV handler which receives context.Context (not gin.Context).
func GetUserIDFromContext(ctx context.Context) int64 {
	if v, ok := ctx.Value("userID").(int64); ok {
		return v
	}
	panic("GetUserIDFromContext: userID not in context")
}
