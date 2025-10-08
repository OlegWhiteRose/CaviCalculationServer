package handler

import (
	"net/http"
	"rip/internal/app/config"
	"rip/internal/app/ds"
	"rip/internal/app/repository"
	"rip/internal/app/storage"
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
		"caviGroup": group,
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

	for i := range groups {
		groups[i].ImageURL = h.Storage.GetImageURLByID(groups[i].ID)
	}

	userID := 3
	groupsInCart := make(map[int]bool)
	cartItemsCount := 0
	calculation, err := h.Repository.GetDraftCalculationByUserID(userID)
	if err == nil {
		calculationGroups, err := h.Repository.GetCalculationGroups(calculation.ID)
		if err == nil {
			for _, calcGroup := range calculationGroups {
				groupsInCart[calcGroup.GroupID] = true
			}
			cartItemsCount = len(calculationGroups)
		}
	}

	ctx.HTML(http.StatusOK, "index.html", gin.H{
		"time":   time.Now().Format("15:04:05"),
		"caviGroups": groups,
		"caviGroupTitle": searchTitle,
		"groupsInCart": groupsInCart,
		"cartItemsCount": cartItemsCount,
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

	for i := range groups {
		groups[i].ImageURL = h.Storage.GetImageURLByID(groups[i].ID)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"caviGroups": groups,
		"caviGroupTitle": searchTitle,
	})
}

func (h *Handler) GetCaviCalculation(ctx *gin.Context) {
	userID := 3

	calculation, err := h.Repository.GetDraftCalculationByUserID(userID)
	if err != nil {
		calculation, err = h.Repository.CreateDraftCalculation(userID)
		if err != nil {
			logrus.Error(err)
			ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to create calculation"})
			return
		}
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
		"caviCalculation": calculation,
		"defaultSystolic": defaultSystolic,
		"defaultDiastolic": defaultDiastolic,
		"defaultPWV": defaultPWV,
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

	userID := 3

	calculation, err := h.Repository.GetDraftCalculationByUserID(userID)
	if err != nil {
		calculation, err = h.Repository.CreateDraftCalculation(userID)
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

	ctx.Redirect(http.StatusFound, "/")
}

func (h *Handler) RemoveGroupFromCalculation(ctx *gin.Context) {
	groupIDStr := ctx.PostForm("group_id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		logrus.Error(err)
		ctx.Redirect(http.StatusFound, "/cavi-calculation")
		return
	}

	userID := 3

	calculation, err := h.Repository.GetDraftCalculationByUserID(userID)
	if err != nil {
		logrus.Error(err)
		ctx.Redirect(http.StatusFound, "/cavi-calculation")
		return
	}

	err = h.Repository.RemoveGroupFromCalculation(calculation.ID, groupID)
	if err != nil {
		logrus.Error(err)
	}

	ctx.Redirect(http.StatusFound, "/cavi-calculation")
}

func (h *Handler) SoftDeleteCalculation(ctx *gin.Context) {
	userID := 3

	calculation, err := h.Repository.GetDraftCalculationByUserID(userID)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Calculation not found"})
		return
	}

	err = h.Repository.SoftDeleteCalculation(calculation.ID)
	if err != nil {
		logrus.Error(err)
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to delete calculation"})
		return
	}

	ctx.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Calculation deleted"})
}
