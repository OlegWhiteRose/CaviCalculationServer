package api

import (
	"log"
	"rip/internal/app/handler"
	"rip/internal/app/repository"
	"rip/internal/app/storage"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func StartServer() {
	log.Println("Starting server")

	repo, err := repository.NewRepository()
	if err != nil {
		logrus.Error("repository initialization error")
	}

	minioStorage, err := storage.NewMinIOStorage()
	if err != nil {
		logrus.Error("MinIO storage initialization error:", err)
	}

	handler := handler.NewHandler(repo, minioStorage)

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./resources")

	r.GET("/", handler.GetOrders)
	r.GET("/order/:id", handler.GetOrder)
	r.GET("/request", handler.GetRequest)

	r.Run()
	log.Println("Server down")
}
