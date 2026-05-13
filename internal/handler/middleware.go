package handler

import (
	"cloudbox/internal/util/crypto"
	"cloudbox/internal/util/response"
	"strings"

	"github.com/gin-gonic/gin"
)

func JWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "missing authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "invalid authorization header")
			c.Abort()
			return
		}

		claims, err := crypto.ParseToken(parts[1])
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

func GetUserID(c *gin.Context) int64 {
	userID, _ := c.Get("userID")
	return userID.(int64)
}

func GetUsername(c *gin.Context) string {
	username, _ := c.Get("username")
	return username.(string)
}

func GetRole(c *gin.Context) string {
	role, _ := c.Get("role")
	return role.(string)
}
