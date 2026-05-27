package handler

import (
	"cloudbox/internal/util/crypto"
	"cloudbox/internal/util/response"
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

		// Fall back to query parameter (for cross-origin downloads)
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

		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
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
