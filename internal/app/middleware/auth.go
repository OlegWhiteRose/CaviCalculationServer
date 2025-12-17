package middleware

import (
	"context"
	"net/http"
	"rip/internal/app/auth"
	redisClient "rip/internal/app/redis"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
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
			c.JSON(http.StatusUnauthorized, gin.H{"message": "authentication token missing"})
			c.Abort()
			return
		}

		// Проверяем JWT в blacklist Redis
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		isBlacklisted, err := m.Redis.CheckJWTInBlacklist(ctx, tokenString)
		if err != nil {
			logrus.Errorf("Redis blacklist check error: %v", err)
		}
		if isBlacklisted {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "token has been revoked"})
			c.Abort()
			return
		}

		claims, err := auth.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "invalid token"})
			c.Abort()
			return
		}

		c.Set("username", claims.Username)
		c.Set("is_doctor", claims.IsDoctor)
		c.Set("jwt_token", tokenString)
		c.Next()
	}
}

func (m *AuthMiddleware) RequireDoctor() gin.HandlerFunc {
	return func(c *gin.Context) {
		isDoctor, exists := c.Get("is_doctor")
		if !exists || !isDoctor.(bool) {
			c.JSON(http.StatusForbidden, gin.H{"message": "doctor role required"})
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
				c.Set("is_doctor", claims.IsDoctor)
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

func IsDoctor(c *gin.Context) bool {
	isDoc, exists := c.Get("is_doctor")
	if !exists {
		return false
	}
	return isDoc.(bool)
}
