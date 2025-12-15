package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

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

// POST добавление в заявку (API)
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
	ctx.JSON(http.StatusCreated, gin.H{"calculation_id": calc.ID})
}

// DELETE удаление из заявки (без PK м-м)
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
	ctx.JSON(http.StatusOK, gin.H{"calculation_id": calc.ID})
}

// PUT изменение количества/порядка/значения в м-м (без PK м-м)
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

// POST удаление из заявки (HTML form)
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
