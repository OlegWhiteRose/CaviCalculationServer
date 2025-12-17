package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// Токен для авторизации асинхронного сервиса
const AsyncServiceToken = "cavi-async-secret-token-8bytes"

// AsyncResultRequest запрос с результатами асинхронного расчёта
type AsyncResultRequest struct {
	Token       string             `json:"token" binding:"required"`
	GroupsCount int                `json:"groups_count"`
	Results     []AsyncGroupResult `json:"results" binding:"required"`
}

// AsyncGroupResult результат расчёта для одной группы
type AsyncGroupResult struct {
	GroupID   int     `json:"group_id" binding:"required"`
	CAVIIndex float64 `json:"cavi_index" binding:"required"`
}

// AsyncCalculateRequest запрос на запуск асинхронного расчёта
// @Description Данные для запуска асинхронного расчёта CAVI
type AsyncCalculateRequest struct {
	CalculationID     int `json:"calculation_id" example:"1"`
	SystolicPressure  int `json:"systolic_pressure" example:"120"`
	DiastolicPressure int `json:"diastolic_pressure" example:"80"`
	PulseWaveVelocity float64 `json:"pulse_wave_velocity" example:"8.5"`
}

// UpdateAsyncResultAPI обновление результатов асинхронного расчёта
// @Summary      Обновить результаты асинхронного расчёта
// @Description  Принимает результаты расчёта CAVI от асинхронного сервиса. Требует токен авторизации.
// @Tags         calculations
// @Accept       json
// @Produce      json
// @Param        id path int true "ID заявки" example(1)
// @Param        request body AsyncResultRequest true "Результаты расчёта"
// @Success      200 {object} SuccessResponse "Результаты обновлены"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      401 {object} ErrorResponse "Неверный токен"
// @Failure      404 {object} ErrorResponse "Заявка не найдена"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /cavi-calculations/{id}/async-result [put]
func (h *Handler) UpdateAsyncResultAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}

	var req AsyncResultRequest
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid json"})
		return
	}

	// Проверка токена (псевдо-авторизация)
	if req.Token != AsyncServiceToken {
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "fail", "message": "invalid token"})
		return
	}

	// Проверяем существование заявки
	calc, err := h.Repository.GetCalculationByID(id)
	if err != nil || calc == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "calculation not found"})
		return
	}

	// Обновляем CAVI индексы для каждой группы
	for _, result := range req.Results {
		if err := h.Repository.UpdateGroupCAVIIndex(id, result.GroupID, result.CAVIIndex); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"status":  "fail",
				"message": "failed to update group " + strconv.Itoa(result.GroupID),
			})
			return
		}
	}

	// Обновляем groups_count в заявке
	groupsCount := req.GroupsCount
	if groupsCount == 0 {
		groupsCount = len(req.Results)
	}
	if err := h.Repository.UpdateCalculationGroupsCount(id, groupsCount); err != nil {
		log.Errorf("Failed to update groups_count for calculation %d: %v", id, err)
	}

	// Завершаем заявку — меняем статус на completed
	if err := h.Repository.CompleteCalculationAsync(id); err != nil {
		log.Errorf("Failed to complete calculation %d: %v", id, err)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":       "ok",
		"message":      "results updated, calculation completed",
		"count":        len(req.Results),
		"groups_count": groupsCount,
	})
}

// TriggerAsyncCalculationAPI запуск асинхронного расчёта
// @Summary      Запустить асинхронный расчёт CAVI
// @Description  Отправляет запрос в асинхронный сервис для расчёта CAVI индексов. Доступно только врачам.
// @Tags         calculations
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "ID заявки" example(1)
// @Success      200 {object} SuccessResponse "Расчёт запущен"
// @Failure      400 {object} ErrorResponse "Неверные данные или заявка не сформирована"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      403 {object} ErrorResponse "Требуется роль врача"
// @Failure      404 {object} ErrorResponse "Заявка не найдена"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /cavi-calculations/{id}/trigger-async [post]
func (h *Handler) TriggerAsyncCalculationAPI(ctx *gin.Context) {
	if !isDoctorLoggedIn(ctx) {
		ctx.JSON(http.StatusForbidden, gin.H{"status": "fail", "message": "doctor role required"})
		return
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}

	calc, err := h.Repository.GetCalculationDetailed(id)
	if err != nil || calc == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "calculation not found"})
		return
	}

	// Получаем группы заявки
	groups, err := h.Repository.GetCalculationGroups(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}

	if len(groups) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "no groups in calculation"})
		return
	}

	// Значения давления по умолчанию
	systolic := 120
	diastolic := 80
	pwv := 8.5

	if calc.SystolicPressure != nil {
		systolic = *calc.SystolicPressure
	}
	if calc.DiastolicPressure != nil {
		diastolic = *calc.DiastolicPressure
	}
	if calc.PulseWaveVelocity != nil {
		pwv = *calc.PulseWaveVelocity
	}

	// Формируем данные для асинхронного сервиса
	groupsData := make([]map[string]interface{}, 0, len(groups))
	for _, g := range groups {
		if g.Group != nil {
			groupData := map[string]interface{}{
				"group_id":  g.GroupID,
				"age_group": g.Group.AgeGroup,
			}
			if g.Group.DiseaseType != nil {
				groupData["disease_type"] = *g.Group.DiseaseType
			}
			groupsData = append(groupsData, groupData)
		}
	}

	// Формируем payload для async сервиса
	payload := map[string]interface{}{
		"calculation_id":      id,
		"systolic_pressure":   systolic,
		"diastolic_pressure":  diastolic,
		"pulse_wave_velocity": pwv,
		"groups":              groupsData,
	}

	// Отправляем запрос в async сервис
	asyncURL := h.Config.AsyncServiceURL + "/api/calculate"
	jsonData, err := json.Marshal(payload)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": "failed to marshal payload"})
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(asyncURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Errorf("Failed to call async service: %v", err)
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "fail",
			"message": "async service unavailable",
			"error":   err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		log.Errorf("Async service returned status %d", resp.StatusCode)
		ctx.JSON(http.StatusBadGateway, gin.H{
			"status":  "fail",
			"message": "async service error",
			"code":    resp.StatusCode,
		})
		return
	}

	// Записываем doctor_login — врач, который запустил расчёт
	doctorLogin := getDoctorLogin(ctx)
	if doctorLogin != "" {
		if err := h.Repository.SetCalculationDoctor(id, doctorLogin); err != nil {
			log.Errorf("Failed to set doctor_login for calculation %d: %v", id, err)
		}
	}

	log.Infof("Async calculation triggered for calculation_id=%d by doctor=%s", id, doctorLogin)

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "async calculation started",
		"doctor":  doctorLogin,
		"payload": payload,
	})
}
