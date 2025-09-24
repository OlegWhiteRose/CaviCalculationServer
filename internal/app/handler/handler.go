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

	searchQuery := ctx.Query("query")
	if searchQuery == "" {
		orders, err = h.Repository.GetOrders()
		if err != nil {
			logrus.Error(err)
		}
	} else {
		orders, err = h.Repository.GetOrdersByTitle(searchQuery)
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
		"query":  searchQuery,
	})
}

func (h *Handler) GetOrdersJSON(ctx *gin.Context) {
	var orders []repository.Order
	var err error

	searchQuery := ctx.Query("query")
	if searchQuery == "" {
		orders, err = h.Repository.GetOrders()
		if err != nil {
			logrus.Error(err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch orders"})
			return
		}
	} else {
		orders, err = h.Repository.GetOrdersByTitle(searchQuery)
		if err != nil {
			logrus.Error(err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search orders"})
			return
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"orders": orders,
		"query":  searchQuery,
	})
}

func (h *Handler) GetRequest(ctx *gin.Context) {
	requests, err := h.Repository.GetRequests()
	if err != nil {
		logrus.Error(err)
	}

	var request repository.Request
	if len(requests) > 0 {
		request = requests[0]
	}

	ctx.HTML(http.StatusOK, "request.html", gin.H{
		"request": request,
	})
}
