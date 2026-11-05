package handler

import (
	"cloudbox/internal/repository"
	"cloudbox/internal/util/crypto"
	"cloudbox/internal/util/response"
	"context"
	"strings"

	"github.com/gin-gonic/gin"
)

func JWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		// Try Authorization header first
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}

		// Fall back to query parameter (for window.open downloads — browser navigation cannot set headers)
		if token == "" {
			token = c.Query("token")
		}

		if token == "" {
			response.Unauthorized(c, "missing authorization header")
			c.Abort()
			return
		}

		claims, err := crypto.ParseToken(token)
		if err != nil {
			response.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}

		// Verify token version matches current user version (invalidates tokens on role/status/password change)
		if repository.DB != nil {
			var currentVersion int64
			if err := repository.DB.WithContext(c.Request.Context()).
				Table("users").Where("id = ?", claims.UserID).
				Select("COALESCE(token_version, 0)").
				Scan(&currentVersion).Error; err == nil {
				if currentVersion != claims.TokenVersion {
					response.Unauthorized(c, "token has been revoked, please re-login")
					c.Abort()
					return
				}
			}
		}

		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("pwdChanged", claims.PasswordChanged)

		// Inject user info into request context for audit logging
		ctx := context.WithValue(c.Request.Context(), "userID", claims.UserID)
		ctx = context.WithValue(ctx, "username", claims.Username)
		ctx = context.WithValue(ctx, "clientIP", c.ClientIP())
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// PasswordChangeRequiredMiddleware blocks access for users who haven't changed their password.
// Must be used after JWTMiddleware.
func PasswordChangeRequiredMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		pwdChanged, exists := c.Get("pwdChanged")
		if exists && !pwdChanged.(bool) {
			response.Error(c, 403, "请先修改密码")
			c.Abort()
			return
		}
		c.Next()
	}
}

// GetUserID retrieves the authenticated user's ID from context.
// Panics if called on a route without JWTMiddleware.
func GetUserID(c *gin.Context) int64 {
	userID, exists := c.Get("userID")
	if !exists {
		panic("GetUserID called on unauthenticated route - missing JWTMiddleware")
	}
	return userID.(int64)
}

// GetUsername retrieves the authenticated username from context.
// Panics if called on a route without JWTMiddleware.
func GetUsername(c *gin.Context) string {
	username, exists := c.Get("username")
	if !exists {
		panic("GetUsername called on unauthenticated route - missing JWTMiddleware")
	}
	return username.(string)
}

// GetRole retrieves the authenticated user's role from context.
// Panics if called on a route without JWTMiddleware.
func GetRole(c *gin.Context) string {
	role, exists := c.Get("role")
	if !exists {
		panic("GetRole called on unauthenticated route - missing JWTMiddleware")
	}
	return role.(string)
}

// AdminMiddleware requires the authenticated user to have admin role.
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role.(string) != "admin" {
			response.Forbidden(c, "admin access required")
			c.Abort()
			return
		}
		c.Next()
	}
}
