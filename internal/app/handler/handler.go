package handler

import (
	"context"
	"net/http"
	"rip/internal/app/config"
	"rip/internal/app/currentuser"
	"rip/internal/app/ds"
	"rip/internal/app/repository"
	"rip/internal/app/storage"
	"rip/internal/app/userstate"
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

type userRegisterReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type userLoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) UsersRegisterAPI(ctx *gin.Context) {
	var req userRegisterReq
	if err := ctx.BindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid input"})
		return
	}
	if u, ok := userstate.Register(req.Username, req.Password); ok {
		ctx.JSON(http.StatusCreated, gin.H{"status": "ok", "data": u})
		return
	}
	ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "username exists"})
}

func (h *Handler) UsersLoginAPI(ctx *gin.Context) {
	var req userLoginReq
	if err := ctx.BindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid input"})
		return
	}
	if u, ok := userstate.Login(req.Username, req.Password); ok {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok", "data": u})
		return
	}
	ctx.JSON(http.StatusUnauthorized, gin.H{"status": "fail", "message": "invalid credentials"})
}

func (h *Handler) UsersLogoutAPI(ctx *gin.Context) {
	userstate.Logout()
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) UsersMeAPI(ctx *gin.Context) {
	if u, ok := userstate.Me(); ok {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok", "data": u})
		return
	}
	ctx.JSON(http.StatusUnauthorized, gin.H{"status": "fail", "message": "not authenticated"})
}

func (h *Handler) AddGroupToDraftFromGroupAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	userLogin := getCreatorLogin()
	calc, err := h.Repository.AddGroupToDraftByUser(userLogin, id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	_ = h.Repository.SetGroupSelected(id, true)
	ctx.JSON(http.StatusCreated, gin.H{"status": "ok", "calculation_id": calc.ID})
}

type moderateReq struct {
	Action string `json:"action"`
}

func (h *Handler) ModerateCalculationAPI(ctx *gin.Context) {
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
	moderatorLogin := getModeratorLogin()
	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}
	if current.Status != ds.StatusFormed {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "модерация доступна только для сформированной заявки"})
		return
	}
	if req.Action == "complete" {
		groups, err := h.Repository.GetCalculationGroups(id)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
			return
		}
		var total float64
		for _, g := range groups {
			total += g.CAVIIndex
		}
		if err := h.Repository.CompleteCalculation(id, moderatorLogin); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"status": "ok", "total_cavi_index": total})
		return
	}
	if err := h.Repository.RejectCalculation(id, moderatorLogin); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

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
		if items[i].Creator != nil {
			items[i].CreatorUsername = items[i].Creator.Username
		}
		if items[i].Moderator != nil {
			items[i].ModeratorUsername = items[i].Moderator.Username
		}
		groups, _ := h.Repository.GetCalculationGroups(items[i].ID)
		items[i].ResultCount = len(groups)
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "data": items})
}

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
	if calc.Creator != nil {
		calc.CreatorUsername = calc.Creator.Username
	}
	if calc.Moderator != nil {
		calc.ModeratorUsername = calc.Moderator.Username
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
	calc.ResultCount = len(groups)
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "data": calc, "default_image": h.Storage.GetDefaultImageURL()})
}

type calcUpdateReq struct {
	SystolicPressure  *int     `json:"systolic_pressure"`
	DiastolicPressure *int     `json:"diastolic_pressure"`
	PulseWaveVelocity *float64 `json:"pulse_wave_velocity"`
}

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

func (h *Handler) FormCalculationAPI(ctx *gin.Context) {
	if !isModeratorLoggedIn() {
		ctx.JSON(http.StatusForbidden, gin.H{"status": "fail", "message": "требуется роль модератора"})
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	userLogin := getCreatorLogin()

	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}
	if current.Status != ds.StatusDraft {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "формирование доступно только для черновика"})
		return
	}

	groups, err := h.Repository.GetCalculationGroups(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	if len(groups) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "в заявке нет услуг"})
		return
	}
	if err := h.Repository.FormCalculation(id, userLogin); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) CompleteCalculationAPI(ctx *gin.Context) {
	if !isModeratorLoggedIn() {
		ctx.JSON(http.StatusForbidden, gin.H{"status": "fail", "message": "требуется роль модератора"})
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	moderatorLogin := getModeratorLogin()

	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}
	if current.Status != ds.StatusFormed {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "завершение доступно только для сформированной заявки"})
		return
	}

	groups, err := h.Repository.GetCalculationGroups(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	var total float64
	for _, g := range groups {
		total += g.CAVIIndex
	}
	if err := h.Repository.CompleteCalculation(id, moderatorLogin); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "total_cavi_index": total})
}

func (h *Handler) RejectCalculationAPI(ctx *gin.Context) {
	if !isModeratorLoggedIn() {
		ctx.JSON(http.StatusForbidden, gin.H{"status": "fail", "message": "требуется роль модератора"})
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	moderatorLogin := getModeratorLogin()

	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}
	if current.Status != ds.StatusFormed {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "отклонение доступно только для сформированной заявки"})
		return
	}
	if err := h.Repository.RejectCalculation(id, moderatorLogin); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) DeleteCalculationAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	userLogin := getCreatorLogin()
	current, err := h.Repository.GetCalculationByID(id)
	if err != nil || current == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}
	if current.Status != ds.StatusDraft || current.CreatorLogin != userLogin {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "удаление доступно только для черновика создателя"})
		return
	}

	_ = h.Repository.UnselectGroupsByCalculation(id)
	if err := h.Repository.SoftDeleteCalculation(id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type mmAddReq struct {
	GroupID int `json:"group_id"`
}

func (h *Handler) AddItemToDraftAPI(ctx *gin.Context) {
	var req mmAddReq
	if err := ctx.BindJSON(&req); err != nil || req.GroupID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid group_id"})
		return
	}
	userLogin := getCreatorLogin()
	calc, err := h.Repository.AddGroupToDraftByUser(userLogin, req.GroupID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	_ = h.Repository.SetGroupSelected(req.GroupID, true)
	ctx.JSON(http.StatusCreated, gin.H{"status": "ok", "calculation_id": calc.ID})
}

type mmDeleteReq struct {
	GroupID int `json:"group_id"`
}

func (h *Handler) RemoveItemFromDraftAPI(ctx *gin.Context) {
	var req mmDeleteReq
	if err := ctx.BindJSON(&req); err != nil || req.GroupID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid group_id"})
		return
	}
	userLogin := getCreatorLogin()
	calc, err := h.Repository.RemoveGroupFromDraftByUser(userLogin, req.GroupID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	_ = h.Repository.SetGroupSelected(req.GroupID, false)
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "calculation_id": calc.ID})
}

type mmUpdateReq struct {
	GroupID   int      `json:"group_id"`
	CAVIIndex *float64 `json:"cavi_index"`
}

func (h *Handler) UpdateItemInDraftAPI(ctx *gin.Context) {
	var req mmUpdateReq
	if err := ctx.BindJSON(&req); err != nil || req.GroupID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid input"})
		return
	}
	userLogin := getCreatorLogin()
	calc, err := h.Repository.GetDraftCalculationByUserLogin(userLogin)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "no draft found"})
		return
	}
	if req.CAVIIndex != nil {
		if err := h.Repository.UpdateCAVIIndex(calc.ID, req.GroupID, *req.CAVIIndex); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
			return
		}
	} else {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "no updatable fields"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) GetGroupsAPI(ctx *gin.Context) {
	filters := repository.GroupFilters{
		Title:    ctx.Query("title"),
		AgeGroup: ctx.Query("age_group"),
		Disease:  ctx.Query("disease_type"),
	}
	groups, err := h.Repository.GetGroupsFiltered(filters)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	// Сортировка по ID в порядке возрастания
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ID < groups[j].ID
	})
	for i := range groups {
		groups[i].ImageURL = h.Storage.GetImageURLByID(groups[i].ID)
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "data": groups, "default_image": h.Storage.GetDefaultImageURL()})
}

func (h *Handler) GetGroupAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	g, err := h.Repository.GetCaviGroup(id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "fail", "message": "not found"})
		return
	}
	g.ImageURL = h.Storage.GetImageURLByID(g.ID)
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "data": g})
}

type groupCreateReq struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	AgeGroup    string  `json:"age_group"`
	DiseaseType *string `json:"disease_type"`
	BasePrice   float64 `json:"base_price"`
}

func (h *Handler) CreateGroupAPI(ctx *gin.Context) {
	if !isModeratorLoggedIn() {
		ctx.JSON(http.StatusForbidden, gin.H{"status": "fail", "message": "требуется роль модератора"})
		return
	}
	var req groupCreateReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid json"})
		return
	}
	if req.Name == "" || req.AgeGroup == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "name and age_group are required"})
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
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	g.ImageURL = h.Storage.GetImageURLByID(g.ID)
	ctx.JSON(http.StatusCreated, gin.H{"status": "ok", "data": g})
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
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	var req groupUpdateReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid json"})
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
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "no fields to update"})
		return
	}
	if err := h.Repository.UpdateGroup(id, updates); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	g, _ := h.Repository.GetCaviGroup(id)
	g.ImageURL = h.Storage.GetImageURLByID(g.ID)
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "data": g})
}

func (h *Handler) DeleteGroupAPI(ctx *gin.Context) {
	if !isModeratorLoggedIn() {
		ctx.JSON(http.StatusForbidden, gin.H{"status": "fail", "message": "требуется роль модератора"})
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	if err := h.Repository.SoftDeleteGroup(id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	_ = h.Storage.DeleteGroupImage(context.Background(), id)
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) UploadGroupImageAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	fileHeader, err := ctx.FormFile("image")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "image is required"})
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	defer f.Close()
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	if err := h.Repository.UpdateGroup(id, map[string]any{"is_deleted": false}); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	if err := h.Storage.UploadGroupImage(ctx.Request.Context(), id, f, fileHeader.Size, contentType); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "image_url": h.Storage.GetImageURLByID(id)})
}

func (h *Handler) GetCartIconAPI(ctx *gin.Context) {
	userLogin := getCreatorLogin()
	calc, err := h.Repository.GetDraftCalculationByUserLogin(userLogin)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok", "calculation_id": 0, "items": 0})
		return
	}
	count, err := h.Repository.CountItemsInDraft(calc.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "calculation_id": calc.ID, "items": count})
}

func NewHandler(cfg *config.Config, r *repository.Repository, s *storage.MinIOStorage) *Handler {
	return &Handler{
		Config:     cfg,
		Repository: r,
		Storage:    s,
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
