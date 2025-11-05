package handler

import (
	"context"
	"net/http"
	"rip/internal/app/config"
	"rip/internal/app/ds"
	"rip/internal/app/middleware"
	redisClient "rip/internal/app/redis"
	"rip/internal/app/repository"
	"rip/internal/app/storage"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Config     *config.Config
	Repository *repository.Repository
	Storage    *storage.MinIOStorage
	Redis      *redisClient.Client
}

// Removed old singleton methods - now using middleware.GetUsername and middleware.IsModerator

// CalculationsListResponse структура ответа со списком заявок
type CalculationsListResponse struct {
	Data []ds.CaviCalculation `json:"data"`
}

// CalculationDetailResponse структура ответа с деталями заявки
type CalculationDetailResponse struct {
	Data ds.CaviCalculation `json:"data"`
}

type userRegisterReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type userLoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Old userstate handlers removed - now using auth.go handlers (Register, Login, Logout, GetCurrentUser)

func (h *Handler) AddGroupToDraftFromGroupAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	userLogin, _ := middleware.GetUsername(ctx)
	calc, err := h.Repository.AddGroupToDraftByUser(userLogin, id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_ = h.Repository.SetGroupSelected(id, true)
	ctx.JSON(http.StatusCreated, gin.H{"calculation_id": calc.ID})
}

type moderateReq struct {
	Action string `json:"action"`
}

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
		ctx.JSON(http.StatusOK, gin.H{})
		return
	}
	if err := h.Repository.RejectCalculation(id, moderatorLogin); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{})
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

	// Фильтруем заявки для обычных пользователей - показываем только свои
	var filteredItems []ds.CaviCalculation
	for _, item := range items {
		// Модератор видит все заявки
		if isModerator.(bool) {
			filteredItems = append(filteredItems, item)
		} else {
			// Обычный пользователь видит только свои
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
	ctx.JSON(http.StatusOK, gin.H{"data": filteredItems})
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
	
	// Проверка прав доступа: обычный пользователь может видеть только свои заявки
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
	ctx.JSON(http.StatusOK, gin.H{"data": calc, "default_image": h.Storage.GetDefaultImageURL()})
}

type calcUpdateReq struct {
	SystolicPressure  *int     `json:"systolic_pressure"`
	DiastolicPressure *int     `json:"diastolic_pressure"`
	PulseWaveVelocity *float64 `json:"pulse_wave_velocity"`
}

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
	ctx.JSON(http.StatusOK, gin.H{})
}

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
	ctx.JSON(http.StatusOK, gin.H{})
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
	ctx.JSON(http.StatusOK, gin.H{})
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
	ctx.JSON(http.StatusOK, gin.H{})
}

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
	ctx.JSON(http.StatusOK, gin.H{})
}

type mmAddReq struct {
	GroupID int `json:"group_id"`
}

func (h *Handler) AddItemToDraftAPI(ctx *gin.Context) {
	var req mmAddReq
	if err := ctx.BindJSON(&req); err != nil || req.GroupID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid group_id"})
		return
	}
	userLogin, _ := middleware.GetUsername(ctx)
	calc, err := h.Repository.AddGroupToDraftByUser(userLogin, req.GroupID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_ = h.Repository.SetGroupSelected(req.GroupID, true)
	ctx.JSON(http.StatusCreated, gin.H{"calculation_id": calc.ID})
}

type mmDeleteReq struct {
	GroupID int `json:"group_id"`
}

func (h *Handler) RemoveItemFromDraftAPI(ctx *gin.Context) {
	var req mmDeleteReq
	if err := ctx.BindJSON(&req); err != nil || req.GroupID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid group_id"})
		return
	}
	userLogin, _ := middleware.GetUsername(ctx)
	calc, err := h.Repository.RemoveGroupFromDraftByUser(userLogin, req.GroupID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	_ = h.Repository.SetGroupSelected(req.GroupID, false)
	ctx.JSON(http.StatusOK, gin.H{"calculation_id": calc.ID})
}

type mmUpdateReq struct {
	GroupID   int      `json:"group_id"`
	CAVIIndex *float64 `json:"cavi_index"`
}

func (h *Handler) UpdateItemInDraftAPI(ctx *gin.Context) {
	var req mmUpdateReq
	if err := ctx.BindJSON(&req); err != nil || req.GroupID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid input"})
		return
	}
	userLogin, _ := middleware.GetUsername(ctx)
	calc, err := h.Repository.GetDraftCalculationByUserLogin(userLogin)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "no draft found"})
		return
	}
	if req.CAVIIndex != nil {
		if err := h.Repository.UpdateCAVIIndex(calc.ID, req.GroupID, *req.CAVIIndex); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
	} else {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "no updatable fields"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{})
}

func (h *Handler) GetGroupsAPI(ctx *gin.Context) {
	filters := repository.GroupFilters{
		Title:    ctx.Query("title"),
		AgeGroup: ctx.Query("age_group"),
		Disease:  ctx.Query("disease_type"),
	}
	groups, err := h.Repository.GetGroupsFiltered(filters)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	// Сортировка по ID в порядке возрастания
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ID < groups[j].ID
	})
	for i := range groups {
		groups[i].ImageURL = h.Storage.GetImageURLByID(groups[i].ID)
	}
	ctx.JSON(http.StatusOK, gin.H{"data": groups, "default_image": h.Storage.GetDefaultImageURL()})
}

func (h *Handler) GetGroupAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	g, err := h.Repository.GetCaviGroup(id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}
	g.ImageURL = h.Storage.GetImageURLByID(g.ID)
	ctx.JSON(http.StatusOK, gin.H{"data": g})
}

type groupCreateReq struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	AgeGroup    string  `json:"age_group"`
	DiseaseType *string `json:"disease_type"`
	BasePrice   float64 `json:"base_price"`
}

func (h *Handler) CreateGroupAPI(ctx *gin.Context) {
	if !middleware.IsModerator(ctx) {
		ctx.JSON(http.StatusForbidden, gin.H{"message": "требуется роль модератора"})
		return
	}
	var req groupCreateReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid json"})
		return
	}
	if req.Name == "" || req.AgeGroup == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "name and age_group are required"})
		return
	}
	g := &ds.CaviGroup{
		Name:        req.Name,
		Description: req.Description,
		AgeGroup:    req.AgeGroup,
		DiseaseType: req.DiseaseType,
		BasePrice:   req.BasePrice,
		IsSelected:  false,
		IsDeleted:   false,
	}
	if err := h.Repository.CreateGroup(g); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	g.ImageURL = h.Storage.GetImageURLByID(g.ID)
	ctx.JSON(http.StatusCreated, gin.H{"data": g})
}

type groupUpdateReq struct {
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	AgeGroup    *string  `json:"age_group"`
	DiseaseType *string  `json:"disease_type"`
	BasePrice   *float64 `json:"base_price"`
	IsSelected  *bool    `json:"is_selected"`
}

func (h *Handler) UpdateGroupAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	var req groupUpdateReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid json"})
		return
	}
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.AgeGroup != nil {
		updates["age_group"] = *req.AgeGroup
	}
	if req.DiseaseType != nil {
		updates["disease_type"] = *req.DiseaseType
	}
	if req.BasePrice != nil {
		updates["base_price"] = *req.BasePrice
	}
	if req.IsSelected != nil {
		updates["is_selected"] = *req.IsSelected
	}
	if len(updates) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "no fields to update"})
		return
	}
	if err := h.Repository.UpdateGroup(id, updates); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	g, _ := h.Repository.GetCaviGroup(id)
	g.ImageURL = h.Storage.GetImageURLByID(g.ID)
	ctx.JSON(http.StatusOK, gin.H{"data": g})
}

func (h *Handler) DeleteGroupAPI(ctx *gin.Context) {
	if !middleware.IsModerator(ctx) {
		ctx.JSON(http.StatusForbidden, gin.H{"message": "требуется роль модератора"})
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	if err := h.Repository.SoftDeleteGroup(id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	_ = h.Storage.DeleteGroupImage(context.Background(), id)
	ctx.JSON(http.StatusOK, gin.H{})
}

func (h *Handler) UploadGroupImageAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	fileHeader, err := ctx.FormFile("image")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "image is required"})
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	defer f.Close()
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	if err := h.Repository.UpdateGroup(id, map[string]any{"is_deleted": false}); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if err := h.Storage.UploadGroupImage(ctx.Request.Context(), id, f, fileHeader.Size, contentType); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"image_url": h.Storage.GetImageURLByID(id)})
}

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

func NewHandler(cfg *config.Config, r *repository.Repository, s *storage.MinIOStorage, redis *redisClient.Client) *Handler {
	return &Handler{
		Config:     cfg,
		Repository: r,
		Storage:    s,
		Redis:      redis,
	}
}

func (h *Handler) GetCaviGroup(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Invalid ID"})
		return
	}

	group, err := h.Repository.GetCaviGroup(id)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Group not found"})
		return
	}

	group.ImageURL = h.Storage.GetImageURLByID(group.ID)

	ctx.HTML(http.StatusOK, "cavi-group.html", gin.H{
		"caviGroup":    group,
		"defaultImage": h.Storage.GetDefaultImageURL(),
	})
}

func (h *Handler) GetCaviGroups(ctx *gin.Context) {
	var groups []ds.CaviGroup
	var err error

	searchTitle := ctx.Query("caviGroupTitle")
	if searchTitle == "" {
		groups, err = h.Repository.GetCaviGroups()
		if err != nil {
			logrus.Error(err)
			ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to fetch groups"})
			return
		}
	} else {
		groups, err = h.Repository.GetCaviGroupsByTitle(searchTitle)
		if err != nil {
			logrus.Error(err)
			ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to search groups"})
			return
		}
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ID < groups[j].ID
	})

	for i := range groups {
		groups[i].ImageURL = h.Storage.GetImageURLByID(groups[i].ID)
	}

	groupsInCart := make(map[int]bool)
	cartItemsCount := 0
	for _, g := range groups {
		if g.IsSelected {
			groupsInCart[g.ID] = true
			cartItemsCount++
		}
	}

	userLogin := "user1"
	calculationID := 0
	if calculation, err := h.Repository.GetDraftCalculationByUserLogin(userLogin); err == nil {
		calculationID = calculation.ID
	}

	ctx.HTML(http.StatusOK, "index.html", gin.H{
		"time":           time.Now().Format("15:04:05"),
		"caviGroups":     groups,
		"caviGroupTitle": searchTitle,
		"groupsInCart":   groupsInCart,
		"cartItemsCount": cartItemsCount,
		"calculationID":  calculationID,
		"defaultImage":   h.Storage.GetDefaultImageURL(),
	})
}

func (h *Handler) GetCaviGroupsJSON(ctx *gin.Context) {
	var groups []ds.CaviGroup
	var err error

	searchTitle := ctx.Query("caviGroupTitle")
	if searchTitle == "" {
		groups, err = h.Repository.GetCaviGroups()
		if err != nil {
			logrus.Error(err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch groups"})
			return
		}
	} else {
		groups, err = h.Repository.GetCaviGroupsByTitle(searchTitle)
		if err != nil {
			logrus.Error(err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search groups"})
			return
		}
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ID < groups[j].ID
	})

	for i := range groups {
		groups[i].ImageURL = h.Storage.GetImageURLByID(groups[i].ID)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"caviGroups":     groups,
		"caviGroupTitle": searchTitle,
		"defaultImage":   h.Storage.GetDefaultImageURL(),
	})
}

func (h *Handler) GetCaviCalculation(ctx *gin.Context) {
	userLogin := "user1"

	calculation, err := h.Repository.CreateDraftCalculation(userLogin)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to create calculation"})
		return
	}

	ctx.Redirect(http.StatusFound, "/calculations/"+strconv.Itoa(calculation.ID))
}

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

			cavi := ds.CalculateCAVI(
				calculationGroups[i].Group,
				defaultSystolic,
				defaultDiastolic,
				defaultPWV,
			)
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

func (h *Handler) AddGroupToCalculation(ctx *gin.Context) {
	groupIDStr := ctx.PostForm("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		logrus.Error(err)
		ctx.Redirect(http.StatusFound, "/")
		return
	}

	userLogin := "user1"

	calculation, err := h.Repository.GetDraftCalculationByUserLogin(userLogin)
	if err != nil {
		calculation, err = h.Repository.CreateDraftCalculation(userLogin)
		if err != nil {
			logrus.Error(err)
			ctx.Redirect(http.StatusFound, "/")
			return
		}
	}

	calculationGroups, err := h.Repository.GetCalculationGroups(calculation.ID)
	if err == nil {
		for _, calcGroup := range calculationGroups {
			if calcGroup.GroupID == groupID {
				ctx.Redirect(http.StatusFound, "/")
				return
			}
		}
	}

	err = h.Repository.AddGroupToCalculation(calculation.ID, groupID)
	if err != nil {
		logrus.Error(err)
	}

	if err := h.Repository.SetGroupSelected(groupID, true); err != nil {
		logrus.Error(err)
	}

	ctx.Redirect(http.StatusFound, "/")
}

func (h *Handler) RemoveGroupFromCalculation(ctx *gin.Context) {
	groupIDStr := ctx.PostForm("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		logrus.Error(err)

		userLogin := "user1"
		if calc, e := h.Repository.GetDraftCalculationByUserLogin(userLogin); e == nil {
			ctx.Redirect(http.StatusFound, "/calculations/"+strconv.Itoa(calc.ID))
			return
		}
		ctx.Redirect(http.StatusFound, "/")
		return
	}

	userLogin := "user1"

	calculation, err := h.Repository.GetDraftCalculationByUserLogin(userLogin)
	if err != nil {
		logrus.Error(err)
		ctx.Redirect(http.StatusFound, "/")
		ctx.Redirect(http.StatusFound, "/cavi-calculation")
		return
	}

	err = h.Repository.RemoveGroupFromCalculation(calculation.ID, groupID)
	if err != nil {
		logrus.Error(err)
	}

	ctx.Redirect(http.StatusFound, "/calculations/"+strconv.Itoa(calculation.ID))
}

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
