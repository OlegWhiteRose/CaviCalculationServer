package handler

import (
	"context"
	"net/http"
	"rip/internal/app/auth"
	"rip/internal/app/ds"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

// === Request/Response structs для Swagger ===

// UserRegisterRequest запрос на регистрацию
// @Description Данные для регистрации нового пользователя
type UserRegisterRequest struct {
	Username string `json:"username" example:"newuser" binding:"required"`
	Password string `json:"password" example:"password123" binding:"required"`
}

// UserLoginRequest запрос на вход
// @Description Данные для входа в систему
type UserLoginRequest struct {
	Username string `json:"username" example:"user1" binding:"required"`
	Password string `json:"password" example:"password" binding:"required"`
}

// UserUpdateRequest запрос на обновление профиля
// @Description Данные для обновления профиля пользователя
type UserUpdateRequest struct {
	Username *string `json:"username" example:"newusername"`
	Password *string `json:"password" example:"newpassword123"`
}

// UserResponse ответ с данными пользователя
// @Description Информация о пользователе
type UserResponse struct {
	Username string `json:"username" example:"user1"`
	IsDoctor bool   `json:"is_doctor" example:"false"`
}

// LoginResponse ответ при успешном входе
// @Description Токены и информация о пользователе после входа
type LoginResponse struct {
	Username     string `json:"username" example:"user1"`
	IsDoctor     bool   `json:"is_doctor" example:"false"`
	Token        string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	RefreshToken string `json:"refresh_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

// ErrorResponse ответ с ошибкой
// @Description Стандартный ответ с ошибкой
type ErrorResponse struct {
	Status  string `json:"status" example:"fail"`
	Message string `json:"message" example:"invalid input"`
}

// SuccessResponse успешный ответ
// @Description Стандартный успешный ответ
type SuccessResponse struct {
	Status string `json:"status" example:"ok"`
}

// UsersRegisterAPI регистрация нового пользователя
// @Summary      Регистрация
// @Description  Создаёт нового пользователя в системе. Пароль хешируется с помощью bcrypt.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        request body UserRegisterRequest true "Данные для регистрации"
// @Success      201 {object} UserResponse "Пользователь создан"
// @Failure      400 {object} ErrorResponse "Неверные данные или пользователь существует"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /users/register [post]
func (h *Handler) UsersRegisterAPI(ctx *gin.Context) {
	var req UserRegisterRequest
	if err := ctx.BindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid input"})
		return
	}

	var existingUser ds.User
	if err := h.Repository.DB().Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "username exists"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": "error creating user"})
		return
	}

	user := ds.User{
		Username: req.Username,
		Password: string(hashedPassword),
		IsDoctor: false,
	}

	if err := h.Repository.DB().Create(&user).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": "error saving user"})
		return
	}

	ctx.JSON(http.StatusCreated, UserResponse{
		Username: user.Username,
		IsDoctor: user.IsDoctor,
	})
}

// UsersLoginAPI вход в систему
// @Summary      Вход
// @Description  Аутентифицирует пользователя и возвращает JWT токены (access + refresh).
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        request body UserLoginRequest true "Данные для входа"
// @Success      200 {object} LoginResponse "Успешный вход"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      401 {object} ErrorResponse "Неверные учётные данные"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /users/login [post]
func (h *Handler) UsersLoginAPI(ctx *gin.Context) {
	var req UserLoginRequest
	if err := ctx.BindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid input"})
		return
	}

	logrus.Infof("Login attempt for user: %s", req.Username)

	var user ds.User
	if err := h.Repository.DB().Where("username = ?", req.Username).First(&user).Error; err != nil {
		logrus.Errorf("Login: user not found: %s", req.Username)
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "fail", "message": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		logrus.Errorf("Login: password mismatch for user: %s", req.Username)
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "fail", "message": "invalid credentials"})
		return
	}

	logrus.Infof("Login: successful login for user: %s", req.Username)

	token, err := auth.GenerateToken(user.Username, user.IsDoctor)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": "error creating token"})
		return
	}

	refreshToken, err := auth.GenerateRefreshToken(user.Username)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": "error creating refresh token"})
		return
	}

	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := h.Redis.SetRefreshToken(c, user.Username, refreshToken, auth.RefreshTokenTTL()); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": "error saving refresh token"})
		return
	}

	ctx.JSON(http.StatusOK, LoginResponse{
		Username:     user.Username,
		IsDoctor:     user.IsDoctor,
		Token:        token,
		RefreshToken: refreshToken,
	})
}

// UsersLogoutAPI выход из системы
// @Summary      Выход
// @Description  Добавляет access токен в blacklist Redis и удаляет refresh токен.
// @Tags         users
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} SuccessResponse "Успешный выход"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Router       /users/logout [post]
func (h *Handler) UsersLogoutAPI(ctx *gin.Context) {
	username := getCreatorLogin(ctx)
	if username == "" {
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "fail", "message": "not authenticated"})
		return
	}

	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Добавляем access токен в blacklist (согласно методичке)
	jwtToken, exists := ctx.Get("jwt_token")
	if exists && jwtToken != nil {
		tokenStr := jwtToken.(string)
		if err := h.Redis.WriteJWTToBlacklist(c, tokenStr, auth.AccessTokenTTL()); err != nil {
			logrus.Errorf("Failed to add token to blacklist: %v", err)
		}
	}

	// Удаляем refresh токен
	if err := h.Redis.DeleteRefreshToken(c, username); err != nil {
		logrus.Errorf("Failed to delete refresh token: %v", err)
	}

	ctx.JSON(http.StatusOK, SuccessResponse{Status: "ok"})
}

// UsersMeAPI получение текущего пользователя
// @Summary      Текущий пользователь
// @Description  Возвращает информацию о текущем авторизованном пользователе.
// @Tags         users
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} UserResponse "Данные пользователя"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      404 {object} ErrorResponse "Пользователь не найден"
// @Router       /users/me [get]
func (h *Handler) UsersMeAPI(ctx *gin.Context) {
	username := getCreatorLogin(ctx)
	if username == "" {
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "fail", "message": "not authenticated"})
		return
	}

	var user ds.User
	if err := h.Repository.DB().Where("username = ?", username).First(&user).Error; err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "user not found"})
		return
	}

	ctx.JSON(http.StatusOK, UserResponse{
		Username: user.Username,
		IsDoctor: user.IsDoctor,
	})
}

// UsersUpdateMeAPI обновление профиля
// @Summary      Обновить профиль
// @Description  Обновляет username и/или пароль текущего пользователя.
// @Tags         users
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body UserUpdateRequest true "Данные для обновления"
// @Success      200 {object} UserResponse "Профиль обновлён"
// @Failure      400 {object} ErrorResponse "Неверные данные или username занят"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      404 {object} ErrorResponse "Пользователь не найден"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /users/me [put]
func (h *Handler) UsersUpdateMeAPI(ctx *gin.Context) {
	username := getCreatorLogin(ctx)
	if username == "" {
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "fail", "message": "not authenticated"})
		return
	}

	var req UserUpdateRequest
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid json"})
		return
	}
	if req.Username == nil && req.Password == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "no fields to update"})
		return
	}

	var user ds.User
	if err := h.Repository.DB().Where("username = ?", username).First(&user).Error; err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "user not found"})
		return
	}

	if req.Username != nil && *req.Username != user.Username {
		var existingUser ds.User
		if err := h.Repository.DB().Where("username = ?", *req.Username).First(&existingUser).Error; err == nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "username already exists"})
			return
		}
		user.Username = *req.Username
	}

	if req.Password != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": "error hashing password"})
			return
		}
		user.Password = string(hashedPassword)
	}

	if err := h.Repository.DB().Save(&user).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": "error saving user"})
		return
	}

	ctx.JSON(http.StatusOK, UserResponse{
		Username: user.Username,
		IsDoctor: user.IsDoctor,
	})
}


// RefreshRequest запрос на обновление токена
// @Description Refresh токен для получения новой пары токенов
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." binding:"required"`
}

// UsersRefreshAPI обновление токенов
// @Summary      Обновить токены
// @Description  Обновляет access и refresh токены по валидному refresh токену.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        request body RefreshRequest true "Refresh токен"
// @Success      200 {object} LoginResponse "Новые токены"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      401 {object} ErrorResponse "Невалидный или истёкший refresh токен"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /users/refresh [post]
func (h *Handler) UsersRefreshAPI(ctx *gin.Context) {
	var req RefreshRequest
	if err := ctx.BindJSON(&req); err != nil || req.RefreshToken == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "refresh_token is required"})
		return
	}

	// Валидируем refresh токен
	claims, err := auth.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "fail", "message": "invalid or expired refresh token"})
		return
	}

	// Проверяем, что refresh токен совпадает с сохранённым в Redis
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	storedToken, err := h.Redis.GetRefreshToken(c, claims.Username)
	if err != nil || storedToken != req.RefreshToken {
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "fail", "message": "refresh token not found or revoked"})
		return
	}

	// Получаем пользователя из БД для актуальных данных
	var user ds.User
	if err := h.Repository.DB().Where("username = ?", claims.Username).First(&user).Error; err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "fail", "message": "user not found"})
		return
	}

	// Генерируем новые токены
	newToken, err := auth.GenerateToken(user.Username, user.IsDoctor)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": "error creating token"})
		return
	}

	newRefreshToken, err := auth.GenerateRefreshToken(user.Username)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": "error creating refresh token"})
		return
	}

	// Сохраняем новый refresh токен
	if err := h.Redis.SetRefreshToken(c, user.Username, newRefreshToken, auth.RefreshTokenTTL()); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": "error saving refresh token"})
		return
	}

	ctx.JSON(http.StatusOK, LoginResponse{
		Username:     user.Username,
		IsDoctor:     user.IsDoctor,
		Token:        newToken,
		RefreshToken: newRefreshToken,
	})
}
