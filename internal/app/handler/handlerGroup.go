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

// === Request/Response structs для Swagger ===

// GroupCreateRequest запрос на создание группы
// @Description Данные для создания новой группы пациентов
type GroupCreateRequest struct {
	Name        string  `json:"name" example:"Молодые пациенты" binding:"required"`
	Description string  `json:"description" example:"Эластичные сосуды с низким уровнем жесткости"`
	AgeGroup    string  `json:"age_group" example:"young" binding:"required" enums:"young,middle,elderly"`
	DiseaseType *string `json:"disease_type" example:"diabetes" enums:"diabetes,hypertension"`
}

// GroupUpdateRequest запрос на обновление группы
// @Description Данные для обновления группы пациентов
type GroupUpdateRequest struct {
	Name        *string `json:"name" example:"Обновлённое название"`
	Description *string `json:"description" example:"Обновлённое описание"`
	AgeGroup    *string `json:"age_group" example:"middle" enums:"young,middle,elderly"`
	DiseaseType *string `json:"disease_type" example:"hypertension" enums:"diabetes,hypertension"`
	IsSelected  *bool   `json:"is_selected" example:"true"`
}

// GroupResponse ответ с данными группы
// @Description Информация о группе пациентов
type GroupResponse struct {
	ID          int     `json:"id" example:"1"`
	Name        string  `json:"name" example:"Молодые пациенты (до 35 лет)"`
	Description string  `json:"description" example:"Эластичные сосуды с низким уровнем жесткости"`
	AgeGroup    string  `json:"age_group" example:"young"`
	DiseaseType *string `json:"disease_type" example:"diabetes"`
	IsSelected  bool    `json:"is_selected" example:"false"`
	IsDeleted   bool    `json:"is_deleted" example:"false"`
	ImageURL    string  `json:"image_url" example:"http://localhost:9000/cavi-images/diagrams/1.jpg"`
}

// ImageUploadResponse ответ при загрузке изображения
// @Description URL загруженного изображения
type ImageUploadResponse struct {
	ImageURL string `json:"image_url" example:"http://localhost:9000/cavi-images/diagrams/1.jpg"`
}

// AddToDraftResponse ответ при добавлении в черновик
// @Description ID заявки-черновика
type AddToDraftResponse struct {
	CalculationID int `json:"calculation_id" example:"1"`
}

// GetGroupsAPI получение списка групп
// @Summary      Список групп
// @Description  Возвращает список всех групп пациентов с возможностью фильтрации по названию, возрастной группе и типу заболевания.
// @Tags         groups
// @Produce      json
// @Param        title query string false "Поиск по названию (частичное совпадение)" example:"Молодые"
// @Param        age_group query string false "Фильтр по возрастной группе" Enums(young, middle, elderly)
// @Param        disease_type query string false "Фильтр по типу заболевания" Enums(diabetes, hypertension)
// @Success      200 {array} GroupResponse "Список групп"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /cavi-groups [get]
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

// GetGroupAPI получение группы по ID
// @Summary      Получить группу
// @Description  Возвращает детальную информацию о группе пациентов по её ID.
// @Tags         groups
// @Produce      json
// @Param        id path int true "ID группы" example(1)
// @Success      200 {object} GroupResponse "Информация о группе"
// @Failure      400 {object} ErrorResponse "Неверный ID"
// @Failure      404 {object} ErrorResponse "Группа не найдена"
// @Router       /cavi-groups/{id} [get]
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

// CreateGroupAPI создание группы
// @Summary      Создать группу
// @Description  Создаёт новую группу пациентов. Доступно только врачам.
// @Tags         groups
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body GroupCreateRequest true "Данные новой группы"
// @Success      201 {object} GroupResponse "Группа создана"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      403 {object} ErrorResponse "Требуется роль врача"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /cavi-groups [post]
func (h *Handler) CreateGroupAPI(ctx *gin.Context) {
	if !isDoctorLoggedIn(ctx) {
		ctx.JSON(http.StatusForbidden, gin.H{"status": "fail", "message": "doctor role required"})
		return
	}
	var req GroupCreateRequest
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

// UpdateGroupAPI обновление группы
// @Summary      Обновить группу
// @Description  Обновляет данные группы пациентов. Доступно только модераторам.
// @Tags         groups
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id path int true "ID группы" example(1)
// @Param        request body GroupUpdateRequest true "Данные для обновления"
// @Success      200 {object} GroupResponse "Группа обновлена"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      403 {object} ErrorResponse "Требуется роль модератора"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /cavi-groups/{id} [put]
func (h *Handler) UpdateGroupAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	var req GroupUpdateRequest
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

// DeleteGroupAPI удаление группы
// @Summary      Удалить группу
// @Description  Мягкое удаление группы (is_deleted = true). Также удаляет изображение. Доступно только врачам.
// @Tags         groups
// @Security     BearerAuth
// @Produce      json
// @Param        id path int true "ID группы" example(1)
// @Success      200 {object} SuccessResponse "Группа удалена"
// @Failure      400 {object} ErrorResponse "Неверный ID"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      403 {object} ErrorResponse "Требуется роль врача"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /cavi-groups/{id} [delete]
func (h *Handler) DeleteGroupAPI(ctx *gin.Context) {
	if !isDoctorLoggedIn(ctx) {
		ctx.JSON(http.StatusForbidden, gin.H{"status": "fail", "message": "doctor role required"})
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
	ctx.JSON(http.StatusOK, SuccessResponse{Status: "ok"})
}

// UploadGroupImageAPI загрузка изображения группы
// @Summary      Загрузить изображение
// @Description  Загружает или заменяет изображение группы. Доступно только модераторам.
// @Tags         groups
// @Security     BearerAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        id path int true "ID группы" example(1)
// @Param        image formData file true "Файл изображения (JPEG, PNG)"
// @Success      200 {object} ImageUploadResponse "Изображение загружено"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      403 {object} ErrorResponse "Требуется роль модератора"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /cavi-groups/{id}/image [post]
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
	ctx.JSON(http.StatusOK, ImageUploadResponse{ImageURL: h.Storage.GetImageURLByID(id)})
}

// AddGroupToDraftFromGroupAPI добавление группы в черновик
// @Summary      Добавить в черновик
// @Description  Добавляет группу в заявку-черновик текущего пользователя. Если черновика нет — создаёт новый.
// @Tags         groups
// @Security     BearerAuth
// @Produce      json
// @Param        id path int true "ID группы" example(1)
// @Success      201 {object} AddToDraftResponse "Группа добавлена в черновик"
// @Failure      400 {object} ErrorResponse "Неверный ID или группа уже в черновике"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Router       /cavi-groups/{id}/add-to-draft [post]
func (h *Handler) AddGroupToDraftFromGroupAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid id"})
		return
	}
	userLogin := getCreatorLogin(ctx)
	calc, err := h.Repository.AddGroupToDraftByUser(userLogin, id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	_ = h.Repository.SetGroupSelected(id, true)
	ctx.JSON(http.StatusCreated, AddToDraftResponse{CalculationID: calc.ID})
}

// === HTML handlers (не для API) ===

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
	} else {
		groups, err = h.Repository.GetCaviGroupsByTitle(searchTitle)
	}
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to fetch groups"})
		return
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
