package handler

import (
	"net/http"
	"rip/internal/app/ds"
	"rip/internal/app/repository"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// === Request/Response structs для Swagger ===

// CalculationUpdateRequest запрос на обновление заявки
// @Description Данные для обновления полей заявки (давление, скорость пульсовой волны)
type CalculationUpdateRequest struct {
	SystolicPressure  *int     `json:"systolic_pressure" example:"120"`
	DiastolicPressure *int     `json:"diastolic_pressure" example:"80"`
	PulseWaveVelocity *float64 `json:"pulse_wave_velocity" example:"8.5"`
}

// ModerateRequest запрос на модерацию заявки
// @Description Действие модератора над заявкой
type ModerateRequest struct {
	Action string `json:"action" example:"complete" enums:"complete,reject" binding:"required"`
}

// CartIconResponse ответ с информацией о корзине
// @Description Информация о черновике заявки пользователя
type CartIconResponse struct {
	CalculationID int `json:"calculation_id" example:"1"`
	Items         int `json:"items" example:"3"`
}

// CalculationGroupResponse группа в заявке
// @Description Информация о группе в составе заявки
type CalculationGroupResponse struct {
	GroupID   int      `json:"group_id" example:"1"`
	CAVIIndex *float64 `json:"cavi_index" example:"7.5"`
	Group     *GroupResponse `json:"group,omitempty"`
}

// CalculationResponse ответ с данными заявки
// @Description Полная информация о заявке на расчёт CAVI
type CalculationResponse struct {
	ID                int                        `json:"id" example:"1"`
	Status            string                     `json:"status" example:"draft"`
	CreatorLogin      string                     `json:"creator_login" example:"user1"`
	ModeratorLogin    *string                    `json:"moderator_login" example:"moderator1"`
	DateCreated       string                     `json:"date_created" example:"2025-01-15T10:30:00Z"`
	DateFormed        *string                    `json:"date_formed" example:"2025-01-15T11:00:00Z"`
	DateCompleted     *string                    `json:"date_completed" example:"2025-01-15T12:00:00Z"`
	SystolicPressure  *int                       `json:"systolic_pressure" example:"120"`
	DiastolicPressure *int                       `json:"diastolic_pressure" example:"80"`
	PulseWaveVelocity *float64                   `json:"pulse_wave_velocity" example:"8.5"`
	GroupsCount       *int                       `json:"groups_count" example:"3"`
	CalculationGroups []CalculationGroupResponse `json:"calculation_groups,omitempty"`
}

type calcUpdateReq struct {
	SystolicPressure  *int     `json:"systolic_pressure"`
	DiastolicPressure *int     `json:"diastolic_pressure"`
	PulseWaveVelocity *float64 `json:"pulse_wave_velocity"`
}

type moderateReq struct {
	Action string `json:"action"`
}

// GetCartIconAPI получение информации о корзине
// @Summary      Корзина (черновик)
// @Description  Возвращает ID черновика заявки текущего пользователя и количество групп в нём.
// @Tags         calculations
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} CartIconResponse "Информация о корзине"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Router       /cavi-calculations/draft [get]
func (h *Handler) GetCartIconAPI(ctx *gin.Context) {
	userLogin := getCreatorLogin(ctx)
	calc, err := h.Repository.GetDraftCalculationByUserLogin(userLogin)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"calculation_id": 0, "items": 0})
		return
	}
	count, err := h.Repository.CountItemsInDraft(calc.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"calculation_id": calc.ID, "items": count})
}

// ListCalculationsAPI получение списка заявок
// @Summary      Список заявок
// @Description  Возвращает список заявок (кроме удалённых и черновиков) с фильтрацией по статусу и диапазону дат формирования.
// @Tags         calculations
// @Security     BearerAuth
// @Produce      json
// @Param        status query string false "Фильтр по статусу" Enums(formed, completed, rejected)
// @Param        date_from query string false "Дата формирования от (YYYY-MM-DD)" example("2025-01-01")
// @Param        date_to query string false "Дата формирования до (YYYY-MM-DD)" example("2025-12-31")
// @Success      200 {array} CalculationResponse "Список заявок"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /cavi-calculations [get]
func (h *Handler) ListCalculationsAPI(ctx *gin.Context) {
	status := ctx.Query("status")
	df := ctx.Query("date_from")
	dt := ctx.Query("date_to")
	var dfPtr, dtPtr *string
	if df != "" {
		dfPtr = &df
	}
	if dt != "" {
		dtPtr = &dt
	}
	items, err := h.Repository.ListCalculationsFiltered(repository.CalculationFilters{Status: status, DateFrom: dfPtr, DateTo: dtPtr})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}

	for i := range items {
		groups, err := h.Repository.GetCalculationGroups(items[i].ID)
		if err == nil {
			items[i].CalculationGroups = groups
		}
	}
	ctx.JSON(http.StatusOK, items)
}

// GetCalculationAPI получение заявки по ID
// @Summary      Получить заявку
// @Description  Возвращает детальную информацию о заявке, включая все группы в её составе.
// @Tags         calculations
// @Security     BearerAuth
// @Produce      json
// @Param        id path int true "ID заявки" example(1)
// @Success      200 {object} CalculationResponse "Информация о заявке"
// @Failure      400 {object} ErrorResponse "Неверный ID"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      404 {object} ErrorResponse "Заявка не найдена"
// @Router       /cavi-calculations/{id} [get]
func (h *Handler) GetCalculationAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	calc, err := h.Repository.GetCalculationDetailed(id)
	if err != nil || calc == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}
	if calc.Status == ds.StatusDeleted {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}

	groups, err := h.Repository.GetCalculationGroups(calc.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Group != nil && groups[j].Group != nil {
			return groups[i].Group.ID < groups[j].Group.ID
		}
		return groups[i].GroupID < groups[j].GroupID
	})

	for i := range groups {
		if groups[i].Group != nil {
			groups[i].Group.ImageURL = h.Storage.GetImageURLByID(groups[i].Group.ID)
		}
	}
	calc.CalculationGroups = groups
	ctx.JSON(http.StatusOK, calc)
}

// UpdateCalculationAPI обновление полей заявки
// @Summary      Обновить заявку
// @Description  Обновляет поля заявки: систолическое/диастолическое давление, скорость пульсовой волны.
// @Tags         calculations
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "ID заявки" example(1)
// @Param        request body CalculationUpdateRequest true "Данные для обновления"
// @Success      200 {object} SuccessResponse "Заявка обновлена"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /cavi-calculations/{id} [put]
func (h *Handler) UpdateCalculationAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	var req calcUpdateReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid json"})
		return
	}
	updates := map[string]any{}
	if req.SystolicPressure != nil {
		updates["systolic_pressure"] = *req.SystolicPressure
	}
	if req.DiastolicPressure != nil {
		updates["diastolic_pressure"] = *req.DiastolicPressure
	}
	if req.PulseWaveVelocity != nil {
		updates["pulse_wave_velocity"] = *req.PulseWaveVelocity
	}
	if len(updates) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "no fields to update"})
		return
	}
	if err := h.Repository.UpdateCalculationAllowed(id, updates); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// FormCalculationAPI формирование заявки
// @Summary      Сформировать заявку
// @Description  Переводит черновик в статус "сформирована". Проверяет наличие групп в заявке. Устанавливает дату формирования.
// @Tags         calculations
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} CalculationResponse "Заявка сформирована"
// @Failure      400 {object} ErrorResponse "Нет групп в заявке"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      404 {object} ErrorResponse "Черновик не найден"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /cavi-calculations/draft/form [put]
func (h *Handler) FormCalculationAPI(ctx *gin.Context) {
	userLogin := getCreatorLogin(ctx)
	current, err := h.Repository.GetDraftCalculationByUserLogin(userLogin)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "draft not found"})
		return
	}

	id := current.ID
	groups, err := h.Repository.GetCalculationGroups(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	if len(groups) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "no services in calculation"})
		return
	}
	if err := h.Repository.FormCalculation(id); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	_ = h.Repository.UnselectGroupsByCalculation(id)

	calc, _ := h.Repository.GetCalculationByID(id)
	groups, _ = h.Repository.GetCalculationGroups(id)
	for i := range groups {
		if groups[i].Group != nil {
			groups[i].Group.ImageURL = h.Storage.GetImageURLByID(groups[i].Group.ID)
		}
	}
	calc.CalculationGroups = groups
	ctx.JSON(http.StatusOK, calc)
}

// ModerateCalculationAPI модерация заявки
// @Summary      Модерировать заявку
// @Description  Завершает или отклоняет сформированную заявку. Доступно только модераторам. При завершении устанавливается количество групп.
// @Tags         calculations
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "ID заявки" example(1)
// @Param        request body ModerateRequest true "Действие модератора"
// @Success      200 {object} CalculationResponse "Заявка обработана"
// @Failure      400 {object} ErrorResponse "Неверные данные или статус заявки"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      403 {object} ErrorResponse "Требуется роль модератора"
// @Failure      404 {object} ErrorResponse "Заявка не найдена"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /cavi-calculations/{id}/moderate [put]
func (h *Handler) ModerateCalculationAPI(ctx *gin.Context) {
	if !isModeratorLoggedIn(ctx) {
		ctx.JSON(http.StatusForbidden, gin.H{"status": "fail", "message": "moderator role required"})
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	var req moderateReq
	if err := ctx.BindJSON(&req); err != nil || (req.Action != "complete" && req.Action != "reject") {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid action"})
		return
	}
	moderatorLogin := getModeratorLogin(ctx)
	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}
	if current.Status != ds.StatusFormed {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "moderation only for formed calculations"})
		return
	}
	if req.Action == "complete" {
		groups, err := h.Repository.GetCalculationGroups(id)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
			return
		}
		if err := h.Repository.CompleteCalculation(id, moderatorLogin); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
			return
		}
		if err := h.Repository.SetGroupsCount(id, len(groups)); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
			return
		}
		calc, _ := h.Repository.GetCalculationByID(id)
		groups, _ = h.Repository.GetCalculationGroups(id)
		for i := range groups {
			if groups[i].Group != nil {
				groups[i].Group.ImageURL = h.Storage.GetImageURLByID(groups[i].Group.ID)
			}
		}
		calc.CalculationGroups = groups
		ctx.JSON(http.StatusOK, calc)
		return
	}
	if err := h.Repository.RejectCalculation(id, moderatorLogin); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	calc, _ := h.Repository.GetCalculationByID(id)
	groups, _ := h.Repository.GetCalculationGroups(id)
	for i := range groups {
		if groups[i].Group != nil {
			groups[i].Group.ImageURL = h.Storage.GetImageURLByID(groups[i].Group.ID)
		}
	}
	calc.CalculationGroups = groups
	ctx.JSON(http.StatusOK, calc)
}

// DeleteCalculationAPI удаление заявки
// @Summary      Удалить заявку
// @Description  Мягкое удаление заявки-черновика. Доступно только создателю заявки. Удаляет связи с группами.
// @Tags         calculations
// @Security     BearerAuth
// @Produce      json
// @Param        id path int true "ID заявки" example(1)
// @Success      200 {object} SuccessResponse "Заявка удалена"
// @Failure      400 {object} ErrorResponse "Неверный ID или заявка не является черновиком"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      404 {object} ErrorResponse "Заявка не найдена"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /cavi-calculations/{id} [delete]
func (h *Handler) DeleteCalculationAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	userLogin := getCreatorLogin(ctx)
	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}
	if current.Status != ds.StatusDraft || current.CreatorLogin != userLogin {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "deletion only for creator's draft"})
		return
	}

	_ = h.Repository.UnselectGroupsByCalculation(id)
	if err := h.Repository.SoftDeleteCalculation(id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetCaviCalculationByID GET одна запись (HTML)
func (h *Handler) GetCaviCalculationByID(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Invalid calculation ID"})
		return
	}

	calculation, err := h.Repository.GetCalculationByID(id)
	if err != nil || calculation == nil {
		logrus.Error(err)
		ctx.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Calculation not found"})
		return
	}

	if calculation.Status == ds.StatusDeleted {
		ctx.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Calculation not found or deleted"})
		return
	}

	defaultSystolic := 120
	defaultDiastolic := 80
	defaultPWV := 8.5

	calculationGroups, err := h.Repository.GetCalculationGroups(calculation.ID)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to fetch calculation groups"})
		return
	}

	sort.Slice(calculationGroups, func(i, j int) bool {
		if calculationGroups[i].Group != nil && calculationGroups[j].Group != nil {
			return calculationGroups[i].Group.ID < calculationGroups[j].Group.ID
		}
		return calculationGroups[i].GroupID < calculationGroups[j].GroupID
	})

	for i := range calculationGroups {
		if calculationGroups[i].Group != nil {
			calculationGroups[i].Group.ImageURL = h.Storage.GetImageURLByID(calculationGroups[i].Group.ID)
			cavi := ds.CalculateCAVI(calculationGroups[i].Group, defaultSystolic, defaultDiastolic, defaultPWV)
			calculationGroups[i].CalculatedCAVI = cavi
		}
	}

	calculation.CalculationGroups = calculationGroups

	ctx.HTML(http.StatusOK, "cavi-calculation.html", gin.H{
		"caviCalculation":  calculation,
		"defaultSystolic":  defaultSystolic,
		"defaultDiastolic": defaultDiastolic,
		"defaultPWV":       defaultPWV,
		"defaultImage":     h.Storage.GetDefaultImageURL(),
	})
}

// SoftDeleteCalculationByID POST удаление заявки (HTML form)
func (h *Handler) SoftDeleteCalculationByID(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Invalid calculation ID"})
		return
	}

	if err := h.Repository.UnselectAllGroups(); err != nil {
		logrus.Error(err)
	}

	err = h.Repository.SoftDeleteCalculation(id)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to delete calculation"})
		return
	}

	ctx.Redirect(http.StatusFound, "/")
}
