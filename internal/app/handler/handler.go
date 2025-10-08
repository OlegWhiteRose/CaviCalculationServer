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

func (h *Handler) GetOrder(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logrus.Error(err)
	}

	order, err := h.Repository.GetOrder(id)
	if err != nil {
		logrus.Error(err)
	}


	order.ImageURL = h.Storage.GetImageURLByID(order.ID)

	ctx.HTML(http.StatusOK, "order.html", gin.H{
		"order": order,
	})
}

func (h *Handler) GetOrders(ctx *gin.Context) {
	var orders []repository.Order
	var err error

    searchTitle := ctx.Query("cohort")
    if searchTitle == "" {
		orders, err = h.Repository.GetOrders()
		if err != nil {
			logrus.Error(err)
		}
	} else {
        orders, err = h.Repository.GetOrdersByTitle(searchTitle)
		if err != nil {
			logrus.Error(err)
		}
	}


	for i := range orders {
		orders[i].ImageURL = h.Storage.GetImageURLByID(orders[i].ID)
	}

    ctx.HTML(http.StatusOK, "index.html", gin.H{
		"time":   time.Now().Format("15:04:05"),
		"orders": orders,
        "cohort": searchTitle,
	})
}

func (h *Handler) GetOrdersJSON(ctx *gin.Context) {
	var orders []repository.Order
	var err error

    searchTitle := ctx.Query("cohort")
    if searchTitle == "" {
		orders, err = h.Repository.GetOrders()
		if err != nil {
			logrus.Error(err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch orders"})
			return
		}
	} else {
        orders, err = h.Repository.GetOrdersByTitle(searchTitle)
		if err != nil {
			logrus.Error(err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search orders"})
			return
		}
	}

	for i := range orders {
		orders[i].ImageURL = h.Storage.GetImageURLByID(orders[i].ID)
	}

    ctx.JSON(http.StatusOK, gin.H{
		"orders": orders,
        "cohort": searchTitle,
	})
}

func (h *Handler) GetCalculation(ctx *gin.Context) {
    requests, err := h.Repository.GetCalculations()
	if err != nil {
		logrus.Error(err)
	}

    var request repository.Calculation
	if len(requests) > 0 {
		request = requests[0]
		for i := range request.Services {
			request.Services[i].ImageURL = h.Storage.GetImageURLByID(request.Services[i].ID)
		}
	}

    ctx.HTML(http.StatusOK, "calculation.html", gin.H{
        "calculation": request,
	})
}
