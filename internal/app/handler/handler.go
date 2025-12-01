package handler

import (
	"rip/internal/app/config"
	redisClient "rip/internal/app/redis"
	"rip/internal/app/repository"
	"rip/internal/app/storage"
)

type Handler struct {
	Config     *config.Config
	Repository *repository.Repository
	Storage    *storage.MinIOStorage
	Redis      *redisClient.Client
}

type ImageUploadResponse struct {
	ImageURL string `json:"image_url" example:"http://localhost:8000/storage/groups/1.jpg"`
}

func NewHandler(cfg *config.Config, r *repository.Repository, s *storage.MinIOStorage, redis *redisClient.Client) *Handler {
	return &Handler{
		Config:     cfg,
		Repository: r,
		Storage:    s,
		Redis:      redis,
	}
}
