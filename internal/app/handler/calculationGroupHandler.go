package handler

import (
	"net/http"
	"rip/internal/app/middleware"
	"strconv"

	"github.com/gin-gonic/gin"
)

type mmAddReq struct {
	GroupID int `json:"group_id"`
}

// AddGroupToDraftFromGroupAPI добавляет группу в черновик
// @Summary      Добавить группу в черновик
// @Description  Добавляет группу пациентов в заявку-черновик пользователя
// @Tags         groups
// @Security     BearerAuth
// @Produce      json
// @Param        id path int true "ID группы"
// @Success      201 {object} AddToDraftResponse "ID заявки"
// @Failure      400 {object} ErrorResponse "Неверный ID"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /api/cavi-groups/{id}/add-to-draft [post]
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

// RemoveItemFromDraftAPI удаляет группу из черновика
// @Summary      Удалить группу из черновика
// @Description  Удаляет группу пациентов из заявки-черновика
// @Tags         many-to-many
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body mmDeleteReq true "ID группы для удаления"
// @Success      200 {object} map[string]int "ID заявки"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /api/cavi-calculations/draft/groups [delete]
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

// UpdateItemInDraftAPI обновляет CAVI индекс группы в черновике
// @Summary      Обновить CAVI индекс в черновике
// @Description  Изменяет значение CAVI индекса для группы в заявке-черновике
// @Tags         many-to-many
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body mmUpdateReq true "ID группы и новый CAVI индекс"
// @Success      200 {object} SuccessResponse "CAVI индекс обновлен"
// @Failure      400 {object} ErrorResponse "Неверные данные"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Failure      500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router       /api/cavi-calculations/draft/groups [put]
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
	ctx.JSON(http.StatusOK, gin.H{"message": "операция выполнена успешно"})
}
