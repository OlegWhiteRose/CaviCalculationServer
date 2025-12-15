package handler

import (
	"rip/internal/app/config"
	"rip/internal/app/currentuser"
	"rip/internal/app/repository"
	"rip/internal/app/storage"
	"rip/internal/app/userstate"
)

type Handler struct {
	Config     *config.Config
	Repository *repository.Repository
	Storage    *storage.MinIOStorage
}

func NewHandler(cfg *config.Config, r *repository.Repository, s *storage.MinIOStorage) *Handler {
	return &Handler{
		Config:     cfg,
		Repository: r,
		Storage:    s,
	}
}

func isModeratorLoggedIn() bool {
	if u, ok := userstate.Me(); ok {
		return u.IsModerator
	}
	return false
}

func getCreatorLogin() string {
	if u, ok := userstate.Me(); ok {
		return u.Username
	}
	return currentuser.CurrentCreatorLogin()
}

func getModeratorLogin() string {
	if u, ok := userstate.Me(); ok {
		return u.Username
	}
	return currentuser.CurrentModeratorLogin()
}
