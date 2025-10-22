package api

import (
	"fmt"
	"log"
	"rip/internal/app/config"
	"rip/internal/app/dsn"
	"rip/internal/app/handler"
	"rip/internal/app/repository"
	"rip/internal/app/storage"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func StartServer() {
	log.Println("Starting server")
	
	_ = godotenv.Load()

	cfg, err := config.NewConfig()
	if err != nil {
		logrus.Fatal("Failed to load config:", err)
	}

	postgresString := dsn.FromEnv()
	fmt.Println(postgresString)

	repo, err := repository.New(postgresString)
	if err != nil {
		logrus.Fatal("Failed to initialize repository:", err)
	}

	minioStorage, err := storage.NewMinIOStorage()
	if err != nil {
		logrus.Error("MinIO storage initialization error:", err)
	}

	handler := handler.NewHandler(cfg, repo, minioStorage)

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./resources")

	r.GET("/", handler.GetCaviGroups)
	r.GET("/cavi-group/:id", handler.GetCaviGroup)
	r.GET("/calculations/:id", handler.GetCaviCalculationByID)
	
	r.POST("/add-group", handler.AddGroupToCalculation)
	r.POST("/calculations/:id/delete", handler.SoftDeleteCalculationByID)

	serverAddr := fmt.Sprintf("%s:%d", cfg.CaviServerHost, cfg.CaviServerPort)
	log.Printf("Server starting on %s", serverAddr)
	r.Run(serverAddr)
	log.Println("Server down")
}
