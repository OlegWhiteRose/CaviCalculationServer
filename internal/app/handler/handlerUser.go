package handler

import (
	"net/http"
	"rip/internal/app/userstate"

	"github.com/gin-gonic/gin"
)

type userRegisterReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userLoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userUpdateReq struct {
	Username *string `json:"username"`
	Password *string `json:"password"`
}

// POST регистрация
func (h *Handler) UsersRegisterAPI(ctx *gin.Context) {
	var req userRegisterReq
	if err := ctx.BindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid input"})
		return
	}
	if u, ok := userstate.Register(req.Username, req.Password); ok {
		ctx.JSON(http.StatusCreated, u)
		return
	}
	ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "username exists"})
}

// POST аутентификация
func (h *Handler) UsersLoginAPI(ctx *gin.Context) {
	var req userLoginReq
	if err := ctx.BindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid input"})
		return
	}
	if u, ok := userstate.Login(req.Username, req.Password); ok {
		ctx.JSON(http.StatusOK, u)
		return
	}
	ctx.JSON(http.StatusUnauthorized, gin.H{"status": "fail", "message": "invalid credentials"})
}

// POST деавторизация
func (h *Handler) UsersLogoutAPI(ctx *gin.Context) {
	userstate.Logout()
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GET полей пользователя после аутентификации (для личного кабинета)
func (h *Handler) UsersMeAPI(ctx *gin.Context) {
	if u, ok := userstate.Me(); ok {
		ctx.JSON(http.StatusOK, u)
		return
	}
	ctx.JSON(http.StatusUnauthorized, gin.H{"status": "fail", "message": "not authenticated"})
}

// PUT пользователя (личный кабинет)
func (h *Handler) UsersUpdateMeAPI(ctx *gin.Context) {
	if _, ok := userstate.Me(); !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "fail", "message": "not authenticated"})
		return
	}
	var req userUpdateReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid json"})
		return
	}
	if req.Username == nil && req.Password == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "no fields to update"})
		return
	}
	u, ok := userstate.UpdateMe(req.Username, req.Password)
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "username already exists"})
		return
	}
	ctx.JSON(http.StatusOK, u)
}
