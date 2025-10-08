package handler

import (
	"net/http"
	"rip/internal/app/repository"
	"rip/internal/app/storage"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Repository *repository.Repository
	Storage    *storage.MinIOStorage
}

func NewHandler(r *repository.Repository, s *storage.MinIOStorage) *Handler {
	return &Handler{
		Repository: r,
		Storage:    s,
	}
}

func (h *Handler) GetCaviGroup(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logrus.Error(err)
	}

	caviGroup, err := h.Repository.GetCaviGroup(id)
	if err != nil {
		logrus.Error(err)
	}


	caviGroup.ImageURL = h.Storage.GetImageURLByID(caviGroup.ID)

	ctx.HTML(http.StatusOK, "cavi-group.html", gin.H{
		"caviGroup": caviGroup,
	})
}

func (h *Handler) GetCaviGroups(ctx *gin.Context) {
	var caviGroups []repository.CaviGroup
	var err error

	searchTitle := ctx.Query("caviGroupTitle")
	if searchTitle == "" {
		caviGroups, err = h.Repository.GetCaviGroups()
		if err != nil {
			logrus.Error(err)
		}
	} else {
		caviGroups, err = h.Repository.GetCaviGroupsByTitle(searchTitle)
		if err != nil {
			logrus.Error(err)
		}
	}


	for i := range caviGroups {
		caviGroups[i].ImageURL = h.Storage.GetImageURLByID(caviGroups[i].ID)
	}

	ctx.HTML(http.StatusOK, "index.html", gin.H{
		"time":   time.Now().Format("15:04:05"),
		"caviGroups": caviGroups,
		"caviGroupTitle": searchTitle,
	})
}

func (h *Handler) GetCaviGroupsJSON(ctx *gin.Context) {
	var caviGroups []repository.CaviGroup
	var err error

	searchTitle := ctx.Query("caviGroupTitle")
	if searchTitle == "" {
		caviGroups, err = h.Repository.GetCaviGroups()
		if err != nil {
			logrus.Error(err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch cavi groups"})
			return
		}
	} else {
		caviGroups, err = h.Repository.GetCaviGroupsByTitle(searchTitle)
		if err != nil {
			logrus.Error(err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search cavi groups"})
			return
		}
	}

	for i := range caviGroups {
		caviGroups[i].ImageURL = h.Storage.GetImageURLByID(caviGroups[i].ID)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"caviGroups": caviGroups,
		"caviGroupTitle": searchTitle,
	})
}

func (h *Handler) GetCaviCalculation(ctx *gin.Context) {
	requests, err := h.Repository.GetCaviCalculations()
	if err != nil {
		logrus.Error(err)
	}

	var request repository.CaviCalculation
	if len(requests) > 0 {
		request = requests[0]
		for i := range request.Services {
			request.Services[i].ImageURL = h.Storage.GetImageURLByID(request.Services[i].ID)
		}
	}

	ctx.HTML(http.StatusOK, "cavi-calculation.html", gin.H{
		"caviCalculation": request,
	})
}
