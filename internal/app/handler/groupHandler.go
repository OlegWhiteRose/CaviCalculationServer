package handler

import (
	"context"
	"net/http"
	"rip/internal/app/ds"
	"rip/internal/app/middleware"
	"rip/internal/app/repository"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
)

type groupCreateReq struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	AgeGroup    string  `json:"age_group"`
	DiseaseType *string `json:"disease_type"`
}

// GetGroupsAPI возвращает список групп пациентов
// @Summary      Получить список групп
// @Description  Возвращает список групп пациентов с возможностью фильтрации
// @Tags         groups
// @Produce      json
// @Param        title query string false "Поиск по названию"
// @Param        age_group query string false "Фильтр по возрастной группе"
// @Param        disease_type query string false "Фильтр по типу заболевания"
// @Success      200 {array} ds.CaviGroup "Список групп"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /api/cavi-groups [get]
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

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ID < groups[j].ID
	})
	for i := range groups {
		groups[i].ImageURL = h.Storage.GetImageURLByID(groups[i].ID)
	}
	ctx.JSON(http.StatusOK, groups)
}

// GetGroupAPI возвращает информацию об одной группе
// @Summary      Получить группу по ID
// @Description  Возвращает детальную информацию о группе пациентов
// @Tags         groups
// @Produce      json
// @Param        id path int true "ID группы"
// @Success      200 {object} ds.CaviGroup "Информация о группе"
// @Failure      400 {object} ErrorResponse "Неверный ID"
// @Failure      404 {object} ErrorResponse "Группа не найдена"
// @Router       /api/cavi-groups/{id} [get]
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
	ctx.JSON(http.StatusOK, g)
}

// CreateGroupAPI создает новую группу пациентов
// @Summary      Создать группу
// @Description  Создает новую группу пациентов (только для модераторов)
// @Tags         groups
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body groupCreateReq true "Данные новой группы"
// @Success      201 {object} ds.CaviGroup "Созданная группа"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      403 {object} ErrorResponse "Требуется роль модератора"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /api/cavi-groups [post]
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
		IsSelected:  false,
		IsDeleted:   false,
	}
	if err := h.Repository.CreateGroup(g); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	g.ImageURL = h.Storage.GetImageURLByID(g.ID)
	ctx.JSON(http.StatusCreated, g)
}

type groupUpdateReq struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	AgeGroup    *string `json:"age_group"`
	DiseaseType *string `json:"disease_type"`
	IsSelected  *bool   `json:"is_selected"`
}

// UpdateGroupAPI обновляет данные группы
// @Summary      Обновить группу
// @Description  Обновляет данные группы пациентов (только для модераторов)
// @Tags         groups
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "ID группы"
// @Param        request body groupUpdateReq true "Данные для обновления"
// @Success      200 {object} ds.CaviGroup "Обновленная группа"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      403 {object} ErrorResponse "Требуется роль модератора"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /api/cavi-groups/{id} [put]
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
	ctx.JSON(http.StatusOK, g)
}

// DeleteGroupAPI удаляет группу
// @Summary      Удалить группу
// @Description  Удаляет группу пациентов (soft-delete, только для модераторов)
// @Tags         groups
// @Security     BearerAuth
// @Produce      json
// @Param        id path int true "ID группы"
// @Success      200 {object} SuccessResponse "Группа удалена"
// @Failure      400 {object} ErrorResponse "Неверный ID"
// @Failure      403 {object} ErrorResponse "Требуется роль модератора"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /api/cavi-groups/{id} [delete]
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
	ctx.JSON(http.StatusOK, gin.H{"message": "операция выполнена успешно"})
}

// UploadGroupImageAPI загружает изображение для группы
// @Summary      Загрузить изображение группы
// @Description  Загружает или обновляет изображение группы (только для модераторов)
// @Tags         groups
// @Security     BearerAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        id path int true "ID группы"
// @Param        image formData file true "Файл изображения"
// @Success      200 {object} ImageUploadResponse "URL изображения"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      403 {object} ErrorResponse "Требуется роль модератора"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /api/cavi-groups/{id}/image [post]
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
