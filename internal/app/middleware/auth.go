package middleware

import (
	"net/http"
	"rip/internal/app/auth"
	redisClient "rip/internal/app/redis"
	"strings"

	"github.com/gin-gonic/gin"
)

type AuthMiddleware struct {
	Redis *redisClient.Client
}

func NewAuthMiddleware(redis *redisClient.Client) *AuthMiddleware {
	return &AuthMiddleware{Redis: redis}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "отсутствует токен аутентификации"})
			c.Abort()
			return
		}

		claims, err := auth.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "невалидный токен"})
			c.Abort()
			return
		}

		c.Set("username", claims.Username)
		c.Set("is_moderator", claims.IsModerator)
		c.Next()
	}
}

func (m *AuthMiddleware) RequireModerator() gin.HandlerFunc {
	return func(c *gin.Context) {
		isModerator, exists := c.Get("is_moderator")
		if !exists || !isModerator.(bool) {
			c.JSON(http.StatusForbidden, gin.H{"message": "требуется роль модератора"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (m *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString != "" {
			claims, err := auth.ValidateToken(tokenString)
			if err == nil {
				c.Set("username", claims.Username)
				c.Set("is_moderator", claims.IsModerator)
			}
		}
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	bearerToken := c.GetHeader("Authorization")
	if len(strings.Split(bearerToken, " ")) == 2 {
		return strings.Split(bearerToken, " ")[1]
	}
	return ""
}

func GetUsername(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		return "", false
	}
	return username.(string), true
}

func IsModerator(c *gin.Context) bool {
	isMod, exists := c.Get("is_moderator")
	if !exists {
		return false
	}
	return isMod.(bool)
}
