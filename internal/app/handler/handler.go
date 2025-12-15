package handler

import (
	"rip/internal/app/config"
	"rip/internal/app/middleware"
	redisClient "rip/internal/app/redis"
	"rip/internal/app/repository"
	"rip/internal/app/storage"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	Config     *config.Config
	Repository *repository.Repository
	Storage    *storage.MinIOStorage
	Redis      *redisClient.Client
}

func NewHandler(cfg *config.Config, r *repository.Repository, s *storage.MinIOStorage, redis *redisClient.Client) *Handler {
	return &Handler{
		Config:     cfg,
		Repository: r,
		Storage:    s,
		Redis:      redis,
	}
}

// isModeratorLoggedIn проверяет, является ли текущий пользователь модератором (через JWT)
func isModeratorLoggedIn(ctx *gin.Context) bool {
	return middleware.IsModerator(ctx)
}

// getCreatorLogin возвращает username текущего пользователя из JWT
func getCreatorLogin(ctx *gin.Context) string {
	username, _ := middleware.GetUsername(ctx)
	return username
}

// getModeratorLogin возвращает username модератора из JWT
func getModeratorLogin(ctx *gin.Context) string {
	username, _ := middleware.GetUsername(ctx)
	return username
}
