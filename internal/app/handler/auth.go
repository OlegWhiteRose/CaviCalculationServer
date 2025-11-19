package handler

import (
	"context"
	"fmt"
	"net/http"
	"rip/internal/app/auth"
	"rip/internal/app/ds"
	"rip/internal/app/middleware"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

// RegisterRequest структура для регистрации
// @Description Данные для регистрации нового пользователя
type RegisterRequest struct {
	Username string `json:"username" binding:"required" example:"newuser"`
	Password string `json:"password" binding:"required" example:"password123"`
}

// LoginRequest структура для логина
// @Description Данные для входа в систему
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"user1"`
	Password string `json:"password" binding:"required" example:"password"`
}

// LoginResponse структура успешного ответа при логине
type LoginResponse struct {
	Message      string `json:"message" example:"аутентификация успешна"`
	Token        string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	RefreshToken string `json:"refresh_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User         struct {
		Username    string `json:"username" example:"user1"`
		IsModerator bool   `json:"is_moderator" example:"false"`
	} `json:"user"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// RegisterResponse структура успешного ответа при регистрации
type RegisterResponse struct {
	Message string `json:"message" example:"пользователь успешно зарегистрирован"`
	User    struct {
		Username    string `json:"username" example:"newuser"`
		IsModerator bool   `json:"is_moderator" example:"false"`
	} `json:"user"`
}

// ErrorResponse структура ответа с ошибкой
type ErrorResponse struct {
	Message string `json:"message" example:"неверные учетные данные"`
}

// SuccessResponse структура успешного ответа
type SuccessResponse struct {
	Message string `json:"message" example:"операция выполнена успешно"`
}

// UserInfoResponse структура ответа с информацией о пользователе
type UserInfoResponse struct {
	Message string `json:"message" example:"данные пользователя"`
	User    struct {
		Username    string `json:"username" example:"user1"`
		IsModerator bool   `json:"is_moderator" example:"false"`
	} `json:"user"`
}

// Register регистрирует нового пользователя
// @Summary      Регистрация нового пользователя
// @Description  Создает нового пользователя в системе
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body RegisterRequest true "Данные для регистрации"
// @Success      201 {object} RegisterResponse "Успешная регистрация"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      409 {object} ErrorResponse "Пользователь уже существует"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /api/auth/register [post]
func (h *Handler) Register(ctx *gin.Context) {
	var req RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "неверные данные"})
		return
	}

	// Проверяем, существует ли пользователь
	var existingUser ds.User
	if err := h.Repository.DB().Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		ctx.JSON(http.StatusConflict, gin.H{"message": "пользователь уже существует"})
		return
	}

	// Хешируем пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "ошибка при создании пользователя"})
		return
	}

	// Создаем пользователя
	user := ds.User{
		Username:    req.Username,
		Password:    string(hashedPassword),
		IsModerator: false,
	}

	if err := h.Repository.DB().Create(&user).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "ошибка при сохранении пользователя"})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message": "пользователь успешно зарегистрирован",
		"user": gin.H{
			"username":     user.Username,
			"is_moderator": user.IsModerator,
		},
	})
}

// Login авторизует пользователя
// @Summary      Вход в систему
// @Description  Аутентифицирует пользователя и возвращает JWT токен
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body LoginRequest true "Данные для входа"
// @Success      200 {object} LoginResponse "Успешный вход"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      401 {object} ErrorResponse "Неверные учетные данные"
// @Router       /api/auth/login [post]
func (h *Handler) Login(ctx *gin.Context) {
	var req LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logrus.Errorf("Login: failed to bind JSON: %v", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "неверные данные"})
		return
	}

	logrus.Infof("Login attempt for user: %s", req.Username)

	// Ищем пользователя
	var user ds.User
	if err := h.Repository.DB().Where("username = ?", req.Username).First(&user).Error; err != nil {
		logrus.Errorf("Login: user not found: %s, error: %v", req.Username, err)
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "неверные учетные данные"})
		return
	}

	logrus.Infof("Login: user found: %s, checking password", req.Username)

	// Проверяем пароль
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		logrus.Errorf("Login: password mismatch for user: %s, error: %v", req.Username, err)
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "неверные учетные данные"})
		return
	}

	logrus.Infof("Login: successful login for user: %s", req.Username)

	// Генерируем JWT токен
	token, err := auth.GenerateToken(user.Username, user.IsModerator)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "ошибка при создании токена"})
		return
	}

	refreshToken, err := auth.GenerateRefreshToken(user.ID, user.Username)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "ошибка при создании refresh токена"})
		return
	}

	refreshKey := fmt.Sprintf("refresh:%d", user.ID)
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := h.Redis.Set(c, refreshKey, refreshToken, auth.RefreshTokenTTL()); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "ошибка при сохранении refresh токена"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":       "аутентификация успешна",
		"token":         token,
		"refresh_token": refreshToken,
		"user": gin.H{
			"username":     user.Username,
			"is_moderator": user.IsModerator,
		},
	})
}

// Logout удаляет сессию пользователя
// @Summary      Выход из системы
// @Description  Удаляет сессию пользователя из Redis
// @Tags         auth
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} SuccessResponse "Успешный выход"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      500 {object} ErrorResponse "Ошибка при удалении сессии"
// @Router       /api/auth/logout [post]
func (h *Handler) Logout(ctx *gin.Context) {
	username, exists := middleware.GetUsername(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "требуется аутентификация"})
		return
	}

	var user ds.User
	if err := h.Repository.DB().Where("username = ?", username).First(&user).Error; err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "пользователь не найден"})
		return
	}

	refreshKey := fmt.Sprintf("refresh:%d", user.ID)
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := h.Redis.Delete(c, refreshKey); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "ошибка при удалении refresh токена"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "успешный выход из системы",
	})
}

// RefreshToken обновляет access токен по refresh токену
// @Summary      Обновить access токен
// @Description  Создает новую пару access/refresh токенов, если refresh токен валиден
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body refreshRequest true "Refresh токен"
// @Success      200 {object} refreshResponse "Новая пара токенов"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      401 {object} ErrorResponse "Refresh токен недействителен"
// @Router       /api/auth/refresh [post]
func (h *Handler) RefreshToken(ctx *gin.Context) {
	var req refreshRequest
	if err := ctx.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "refresh_token обязателен"})
		return
	}

	claims, err := auth.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "refresh-token недействителен"})
		return
	}

	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	refreshKey := fmt.Sprintf("refresh:%d", claims.UserID)
	storedToken, err := h.Redis.Get(c, refreshKey)
	if err != nil || storedToken != req.RefreshToken {
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "refresh-token не найден"})
		return
	}

	var user ds.User
	if err := h.Repository.DB().Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "пользователь не найден"})
		return
	}

	accessToken, err := auth.GenerateToken(user.Username, user.IsModerator)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "не удалось создать access токен"})
		return
	}

	newRefreshToken, err := auth.GenerateRefreshToken(user.ID, user.Username)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "не удалось создать refresh токен"})
		return
	}

	if err := h.Redis.Set(c, refreshKey, newRefreshToken, auth.RefreshTokenTTL()); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "ошибка при обновлении refresh токена"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
	})
}

// GetCurrentUser возвращает информацию о текущем пользователе
// @Summary      Получить текущего пользователя
// @Description  Возвращает информацию о текущем авторизованном пользователе
// @Tags         auth
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} UserInfoResponse "Информация о пользователе"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      404 {object} ErrorResponse "Пользователь не найден"
// @Router       /api/auth/me [get]
func (h *Handler) GetCurrentUser(ctx *gin.Context) {
	username, exists := middleware.GetUsername(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "требуется аутентификация"})
		return
	}

	var user ds.User
	if err := h.Repository.DB().Where("username = ?", username).First(&user).Error; err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"message": "пользователь не найден"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "данные пользователя",
		"user": gin.H{
			"username":     user.Username,
			"is_moderator": user.IsModerator,
		},
	})
}

// UpdateProfileRequest структура для обновления профиля
type UpdateProfileRequest struct {
	Username string `json:"username" example:"newusername"`
	Password string `json:"password" example:"newpassword123"`
}

// UpdateProfileResponse структура ответа при обновлении профиля
type UpdateProfileResponse struct {
	Message string `json:"message" example:"профиль успешно обновлен"`
	User    struct {
		Username    string `json:"username" example:"newusername"`
		IsModerator bool   `json:"is_moderator" example:"false"`
	} `json:"user"`
}

// UpdateProfile обновляет профиль текущего пользователя
// @Summary      Обновить профиль
// @Description  Обновляет username и/или пароль текущего пользователя
// @Tags         auth
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body UpdateProfileRequest true "Данные для обновления"
// @Success      200 {object} UpdateProfileResponse "Профиль успешно обновлен"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      409 {object} ErrorResponse "Имя пользователя уже занято"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /api/auth/me [put]
func (h *Handler) UpdateProfile(ctx *gin.Context) {
	username, exists := middleware.GetUsername(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "требуется аутентификация"})
		return
	}

	var req UpdateProfileRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "неверные данные"})
		return
	}

	// Получаем текущего пользователя
	var user ds.User
	if err := h.Repository.DB().Where("username = ?", username).First(&user).Error; err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"message": "пользователь не найден"})
		return
	}

	// Обновляем username если указан
	if req.Username != "" && req.Username != user.Username {
		// Проверяем, не занято ли новое имя
		var existingUser ds.User
		if err := h.Repository.DB().Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
			ctx.JSON(http.StatusConflict, gin.H{"message": "имя пользователя уже занято"})
			return
		}
		user.Username = req.Username
	}

	// Обновляем пароль если указан
	if req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": "ошибка при хешировании пароля"})
			return
		}
		user.Password = string(hashedPassword)
	}

	// Сохраняем изменения
	if err := h.Repository.DB().Save(&user).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "ошибка при сохранении пользователя"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "профиль успешно обновлен",
		"user": gin.H{
			"username":     user.Username,
			"is_moderator": user.IsModerator,
		},
	})
}
