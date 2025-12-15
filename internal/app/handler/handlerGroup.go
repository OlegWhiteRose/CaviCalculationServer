package handler

import (
	"context"
	"net/http"
	"rip/internal/app/ds"
	"rip/internal/app/repository"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type groupCreateReq struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	AgeGroup    string  `json:"age_group"`
	DiseaseType *string `json:"disease_type"`
}

type groupUpdateReq struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	AgeGroup    *string `json:"age_group"`
	DiseaseType *string `json:"disease_type"`
	IsSelected  *bool   `json:"is_selected"`
}

// GET список с фильтрацией
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
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ID < groups[j].ID
	})
	for i := range groups {
		groups[i].ImageURL = h.Storage.GetImageURLByID(groups[i].ID)
	}
	ctx.JSON(http.StatusOK, groups)
}

// GET одна запись
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
	ctx.JSON(http.StatusOK, g)
}

// POST добавление (без изображения)
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
		IsSelected:  false,
		IsDeleted:   false,
	}
	if err := h.Repository.CreateGroup(g); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	g.ImageURL = h.Storage.GetImageURLByID(g.ID)
	ctx.JSON(http.StatusCreated, g)
}

// PUT изменение
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
	ctx.JSON(http.StatusOK, g)
}

// DELETE удаление. Удаление изображения встроено в метод удаления услуги
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

// POST добавление изображения. Добавление изображения по id услуги, старое изображение заменяется/удаляется
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
	ctx.JSON(http.StatusOK, gin.H{"image_url": h.Storage.GetImageURLByID(id)})
}

// POST добавления в заявку-черновик
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
	ctx.JSON(http.StatusCreated, gin.H{"calculation_id": calc.ID})
}

// GET одна запись (HTML)
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

// GET список (HTML)
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

// GET список (JSON)
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
	})
}

// POST добавление в заявку (HTML form)
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
