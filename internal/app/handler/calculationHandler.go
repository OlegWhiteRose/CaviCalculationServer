package handler

import (
	"net/http"
	"rip/internal/app/ds"
	"rip/internal/app/middleware"
	"rip/internal/app/repository"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
)

type CalculationsListResponse struct {
	Data []ds.CaviCalculation `json:"data"`
}

type CalculationDetailResponse struct {
	Data ds.CaviCalculation `json:"data"`
}

type CartIconResponse struct {
	CalculationID int `json:"calculation_id" example:"1"`
	Items         int `json:"items" example:"3"`
}

type AddToDraftResponse struct {
	CalculationID int `json:"calculation_id" example:"1"`
}

type moderateReq struct {
	Action string `json:"action"`
}

// ModerateCalculationAPI завершает или отклоняет заявку
// @Summary      Завершить/отклонить заявку
// @Description  Изменяет статус заявки с formed на completed или rejected (только для модераторов)
// @Tags         calculations
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "ID заявки"
// @Param        request body moderateReq true "Действие (complete или reject)"
// @Success      200 {object} SuccessResponse "Заявка обработана"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      403 {object} ErrorResponse "Требуется роль модератора"
// @Failure      404 {object} ErrorResponse "Заявка не найдена"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /api/cavi-calculations/{id}/moderate [put]
func (h *Handler) ModerateCalculationAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	var req moderateReq
	if err := ctx.BindJSON(&req); err != nil || (req.Action != "complete" && req.Action != "reject") {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid action"})
		return
	}
	moderatorLogin, _ := middleware.GetUsername(ctx)
	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	if current.Status != ds.StatusFormed {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "модерация доступна только для сформированной заявки"})
		return
	}
	if req.Action == "complete" {
		if err := h.Repository.CompleteCalculation(id, moderatorLogin); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"message": "операция выполнена успешно"})
		return
	}
	if err := h.Repository.RejectCalculation(id, moderatorLogin); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "операция выполнена успешно"})
}

// ListCalculationsAPI возвращает список заявок
// @Summary      Получить список заявок
// @Description  Возвращает список заявок. Обычные пользователи видят только свои заявки, модераторы - все
// @Tags         calculations
// @Security     BearerAuth
// @Produce      json
// @Param        status query string false "Фильтр по статусу"
// @Param        date_from query string false "Дата от (YYYY-MM-DD)"
// @Param        date_to query string false "Дата до (YYYY-MM-DD)"
// @Success      200 {object} CalculationsListResponse "Список заявок"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /api/cavi-calculations [get]
func (h *Handler) ListCalculationsAPI(ctx *gin.Context) {
	username, _ := ctx.Get("username")
	isModerator, _ := ctx.Get("is_moderator")

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
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	var filteredItems []ds.CaviCalculation
	for _, item := range items {
		if isModerator.(bool) {
			filteredItems = append(filteredItems, item)
		} else {
			if item.CreatorLogin == username.(string) {
				filteredItems = append(filteredItems, item)
			}
		}
	}

	for i := range filteredItems {
		if filteredItems[i].Creator != nil {
			filteredItems[i].CreatorUsername = filteredItems[i].Creator.Username
		}
		if filteredItems[i].Moderator != nil {
			filteredItems[i].ModeratorUsername = filteredItems[i].Moderator.Username
		}
		groups, _ := h.Repository.GetCalculationGroups(filteredItems[i].ID)
		filteredItems[i].ResultCount = len(groups)
	}
	ctx.JSON(http.StatusOK, filteredItems)
}

// GetCalculationAPI возвращает детальную информацию о заявке
// @Summary      Получить заявку по ID
// @Description  Возвращает детальную информацию о заявке. Обычные пользователи могут видеть только свои заявки
// @Tags         calculations
// @Security     BearerAuth
// @Produce      json
// @Param        id path int true "ID заявки"
// @Success      200 {object} CalculationDetailResponse "Информация о заявке"
// @Failure      400 {object} ErrorResponse "Неверный ID"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      403 {object} ErrorResponse "Доступ запрещен"
// @Failure      404 {object} ErrorResponse "Заявка не найдена"
// @Router       /api/cavi-calculations/{id} [get]
func (h *Handler) GetCalculationAPI(ctx *gin.Context) {
	username, _ := ctx.Get("username")
	isModerator, _ := ctx.Get("is_moderator")

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	calc, err := h.Repository.GetCalculationDetailed(id)
	if err != nil || calc == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	if calc.Status == ds.StatusDeleted {
		ctx.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}

	if !isModerator.(bool) && calc.CreatorLogin != username.(string) {
		ctx.JSON(http.StatusForbidden, gin.H{"message": "доступ запрещен"})
		return
	}
	if calc.Creator != nil {
		calc.CreatorUsername = calc.Creator.Username
	}
	if calc.Moderator != nil {
		calc.ModeratorUsername = calc.Moderator.Username
	}
	groups, err := h.Repository.GetCalculationGroups(calc.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
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
	calc.ResultCount = len(groups)
	ctx.JSON(http.StatusOK, calc)
}

type calcUpdateReq struct {
	SystolicPressure  *int     `json:"systolic_pressure"`
	DiastolicPressure *int     `json:"diastolic_pressure"`
	PulseWaveVelocity *float64 `json:"pulse_wave_velocity"`
}

// UpdateCalculationAPI обновляет заявку
// @Summary      Обновить заявку
// @Description  Обновляет поля заявки (систолическое давление, диастолическое давление, скорость пульсовой волны)
// @Tags         calculations
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "ID заявки"
// @Param        request body calcUpdateReq true "Данные для обновления"
// @Success      200 {object} ds.CaviCalculation "Обновленная заявка"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /api/cavi-calculations/{id} [put]
func (h *Handler) UpdateCalculationAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	var req calcUpdateReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid json"})
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
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "no fields to update"})
		return
	}
	if err := h.Repository.UpdateCalculationAllowed(id, updates); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	calc, err := h.Repository.GetCalculationDetailed(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if calc.Creator != nil {
		calc.CreatorUsername = calc.Creator.Username
	}
	if calc.Moderator != nil {
		calc.ModeratorUsername = calc.Moderator.Username
	}
	groups, _ := h.Repository.GetCalculationGroups(calc.ID)
	calc.ResultCount = len(groups)
	ctx.JSON(http.StatusOK, calc)
}

// FormCalculationAPI формирует заявку
// @Summary      Сформировать заявку
// @Description  Изменяет статус заявки с draft на formed (только для модераторов)
// @Tags         calculations
// @Security     BearerAuth
// @Produce      json
// @Param        id path int true "ID заявки"
// @Success      200 {object} SuccessResponse "Заявка сформирована"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      403 {object} ErrorResponse "Требуется роль модератора"
// @Failure      404 {object} ErrorResponse "Заявка не найдена"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /api/cavi-calculations/{id}/form [put]
func (h *Handler) FormCalculationAPI(ctx *gin.Context) {
	if !middleware.IsModerator(ctx) {
		ctx.JSON(http.StatusForbidden, gin.H{"message": "требуется роль модератора"})
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	userLogin, _ := middleware.GetUsername(ctx)

	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	if current.Status != ds.StatusDraft {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "формирование доступно только для черновика"})
		return
	}

	groups, err := h.Repository.GetCalculationGroups(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if len(groups) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "в заявке нет услуг"})
		return
	}
	if err := h.Repository.FormCalculation(id, userLogin); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "операция выполнена успешно"})
}

func (h *Handler) CompleteCalculationAPI(ctx *gin.Context) {
	if !middleware.IsModerator(ctx) {
		ctx.JSON(http.StatusForbidden, gin.H{"message": "требуется роль модератора"})
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	moderatorLogin, _ := middleware.GetUsername(ctx)

	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	if current.Status != ds.StatusFormed {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "завершение доступно только для сформированной заявки"})
		return
	}

	if err := h.Repository.CompleteCalculation(id, moderatorLogin); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "операция выполнена успешно"})
}

func (h *Handler) RejectCalculationAPI(ctx *gin.Context) {
	if !middleware.IsModerator(ctx) {
		ctx.JSON(http.StatusForbidden, gin.H{"message": "требуется роль модератора"})
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	moderatorLogin, _ := middleware.GetUsername(ctx)

	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	if current.Status != ds.StatusFormed {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "отклонение доступно только для сформированной заявки"})
		return
	}
	if err := h.Repository.RejectCalculation(id, moderatorLogin); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "операция выполнена успешно"})
}

// DeleteCalculationAPI удаляет заявку
// @Summary      Удалить заявку
// @Description  Удаляет заявку-черновик (доступно только создателю)
// @Tags         calculations
// @Security     BearerAuth
// @Produce      json
// @Param        id path int true "ID заявки"
// @Success      200 {object} SuccessResponse "Заявка удалена"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      404 {object} ErrorResponse "Заявка не найдена"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /api/cavi-calculations/{id} [delete]
func (h *Handler) DeleteCalculationAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	userLogin, _ := middleware.GetUsername(ctx)
	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	if current.Status != ds.StatusDraft || current.CreatorLogin != userLogin {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "удаление доступно только для черновика создателя"})
		return
	}

	_ = h.Repository.UnselectGroupsByCalculation(id)
	if err := h.Repository.SoftDeleteCalculation(id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "операция выполнена успешно"})
}

// GetCartIconAPI возвращает информацию о корзине
// @Summary      Получить иконку корзины
// @Description  Возвращает ID черновика и количество групп в нем
// @Tags         calculations
// @Security     BearerAuth
// @Produce      json
// @Success      200 {object} CartIconResponse "Данные корзины"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Router       /api/cavi-calculations/draft [get]
func (h *Handler) GetCartIconAPI(ctx *gin.Context) {
	userLogin, _ := middleware.GetUsername(ctx)
	calc, err := h.Repository.GetDraftCalculationByUserLogin(userLogin)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"calculation_id": 0, "items": 0})
		return
	}
	count, err := h.Repository.CountItemsInDraft(calc.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"calculation_id": calc.ID, "items": count})
}
