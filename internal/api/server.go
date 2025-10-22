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

	api := r.Group("/api")
	{
		api.GET("/cavi-groups", handler.GetGroupsAPI)
		api.GET("/cavi-groups/:id", handler.GetGroupAPI)
		api.POST("/cavi-groups", handler.CreateGroupAPI)
		api.PUT("/cavi-groups/:id", handler.UpdateGroupAPI)
		api.DELETE("/cavi-groups/:id", handler.DeleteGroupAPI)
		api.POST("/cavi-groups/:id/image", handler.UploadGroupImageAPI)
		api.POST("/cavi-groups/:id/add-to-draft", handler.AddGroupToDraftFromGroupAPI)

		api.GET("/cavi-calculations/draft", handler.GetCartIconAPI)
		api.GET("/cavi-calculations", handler.ListCalculationsAPI)
		api.GET("/cavi-calculations/:id", handler.GetCalculationAPI)
		api.PUT("/cavi-calculations/:id", handler.UpdateCalculationAPI)
		api.PUT("/cavi-calculations/:id/form", handler.FormCalculationAPI)
		api.PUT("/cavi-calculations/:id/moderate", handler.ModerateCalculationAPI)
		api.DELETE("/cavi-calculations/:id", handler.DeleteCalculationAPI)

		api.DELETE("/cavi-calculations/draft/groups", handler.RemoveItemFromDraftAPI)
		api.PUT("/cavi-calculations/draft/groups", handler.UpdateItemInDraftAPI)

		api.POST("/users/register", handler.UsersRegisterAPI)
		api.POST("/users/login", handler.UsersLoginAPI)
		api.POST("/users/logout", handler.UsersLogoutAPI)
		api.GET("/users/me", handler.UsersMeAPI)
	}

	serverAddr := fmt.Sprintf("%s:%d", cfg.CaviServerHost, cfg.CaviServerPort)
	log.Printf("Server starting on %s", serverAddr)
	r.Run(serverAddr)
	log.Println("Server down")
}
