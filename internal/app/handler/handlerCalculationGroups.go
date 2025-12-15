package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// === Request/Response structs для Swagger ===

// DraftGroupAddRequest запрос на добавление группы в черновик
// @Description ID группы для добавления в черновик заявки
type DraftGroupAddRequest struct {
	GroupID int `json:"group_id" example:"1" binding:"required"`
}

// DraftGroupDeleteRequest запрос на удаление группы из черновика
// @Description ID группы для удаления из черновика заявки
type DraftGroupDeleteRequest struct {
	GroupID int `json:"group_id" example:"1" binding:"required"`
}

// DraftGroupUpdateRequest запрос на обновление группы в черновике
// @Description Данные для обновления группы в черновике (индекс CAVI)
type DraftGroupUpdateRequest struct {
	GroupID   int      `json:"group_id" example:"1" binding:"required"`
	CAVIIndex *float64 `json:"cavi_index" example:"7.5"`
}

// DraftGroupResponse ответ с ID заявки
// @Description ID заявки-черновика после операции
type DraftGroupResponse struct {
	CalculationID int `json:"calculation_id" example:"1"`
}

type mmAddReq struct {
	GroupID int `json:"group_id"`
}

type mmDeleteReq struct {
	GroupID int `json:"group_id"`
}

type mmUpdateReq struct {
	GroupID   int      `json:"group_id"`
	CAVIIndex *float64 `json:"cavi_index"`
}

// AddItemToDraftAPI добавление группы в черновик (через body)
// Этот метод не используется в API напрямую, используйте POST /cavi-groups/{id}/add-to-draft
func (h *Handler) AddItemToDraftAPI(ctx *gin.Context) {
	var req mmAddReq
	if err := ctx.BindJSON(&req); err != nil || req.GroupID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid group_id"})
		return
	}
	userLogin := getCreatorLogin(ctx)
	calc, err := h.Repository.AddGroupToDraftByUser(userLogin, req.GroupID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	_ = h.Repository.SetGroupSelected(req.GroupID, true)
	ctx.JSON(http.StatusCreated, gin.H{"calculation_id": calc.ID})
}

// RemoveItemFromDraftAPI удаление группы из черновика
// @Summary      Удалить группу из черновика
// @Description  Удаляет группу из заявки-черновика текущего пользователя.
// @Tags         calculation-groups
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body DraftGroupDeleteRequest true "ID группы"
// @Success      200 {object} DraftGroupResponse "Группа удалена"
// @Failure      400 {object} ErrorResponse "Неверный ID или группа не найдена в черновике"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Router       /cavi-calculations/draft/groups [delete]
func (h *Handler) RemoveItemFromDraftAPI(ctx *gin.Context) {
	var req mmDeleteReq
	if err := ctx.BindJSON(&req); err != nil || req.GroupID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid group_id"})
		return
	}
	userLogin := getCreatorLogin(ctx)
	calc, err := h.Repository.RemoveGroupFromDraftByUser(userLogin, req.GroupID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": err.Error()})
		return
	}
	_ = h.Repository.SetGroupSelected(req.GroupID, false)
	ctx.JSON(http.StatusOK, gin.H{"calculation_id": calc.ID})
}

// UpdateItemInDraftAPI обновление группы в черновике
// @Summary      Обновить группу в черновике
// @Description  Обновляет индекс CAVI для группы в заявке-черновике текущего пользователя.
// @Tags         calculation-groups
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body DraftGroupUpdateRequest true "Данные для обновления"
// @Success      200 {object} SuccessResponse "Группа обновлена"
// @Failure      400 {object} ErrorResponse "Неверные данные или черновик не найден"
// @Failure      401 {object} ErrorResponse "Требуется аутентификация"
// @Router       /cavi-calculations/draft/groups [put]
func (h *Handler) UpdateItemInDraftAPI(ctx *gin.Context) {
	var req mmUpdateReq
	if err := ctx.BindJSON(&req); err != nil || req.GroupID <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "fail", "message": "invalid input"})
		return
	}
	userLogin := getCreatorLogin(ctx)
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

// RemoveGroupFromCalculation POST удаление из заявки (HTML form)
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
		return
	}

	err = h.Repository.RemoveGroupFromCalculation(calculation.ID, groupID)
	if err != nil {
		logrus.Error(err)
	}

	ctx.Redirect(http.StatusFound, "/calculations/"+strconv.Itoa(calculation.ID))
}
