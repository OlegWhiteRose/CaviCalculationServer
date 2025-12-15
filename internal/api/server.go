package api

import (
	"fmt"
	"log"
	_ "rip/docs"
	"rip/internal/app/config"
	"rip/internal/app/dsn"
	"rip/internal/app/handler"
	"rip/internal/app/middleware"
	redisClient "rip/internal/app/redis"
	"rip/internal/app/repository"
	"rip/internal/app/storage"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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

	redis, err := redisClient.NewRedisClient()
	if err != nil {
		logrus.Fatal("Failed to initialize Redis:", err)
	}
	log.Println("Redis connected successfully")

	h := handler.NewHandler(cfg, repo, minioStorage, redis)

	authMiddleware := middleware.NewAuthMiddleware(redis)

	r := gin.Default()

	r.Use(middleware.CORSMiddleware())

	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./resources")

	r.GET("/", h.GetCaviGroups)
	r.GET("/cavi-group/:id", h.GetCaviGroup)
	r.GET("/calculations/:id", h.GetCaviCalculationByID)

	r.POST("/add-group", h.AddGroupToCalculation)
	r.POST("/calculations/:id/delete", h.SoftDeleteCalculationByID)

	// Swagger UI
	log.Println("Registering Swagger UI at /swagger/*any")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")
	{
		api.POST("/users/register", h.UsersRegisterAPI)
		api.POST("/users/login", h.UsersLoginAPI)
		api.POST("/users/logout", authMiddleware.RequireAuth(), h.UsersLogoutAPI)
		api.POST("/users/refresh", h.UsersRefreshAPI)
		api.GET("/users/me", authMiddleware.RequireAuth(), h.UsersMeAPI)
		api.PUT("/users/me", authMiddleware.RequireAuth(), h.UsersUpdateMeAPI)

		api.GET("/cavi-groups", h.GetGroupsAPI)
		api.GET("/cavi-groups/:id", h.GetGroupAPI)
		api.POST("/cavi-groups", authMiddleware.RequireAuth(), authMiddleware.RequireModerator(), h.CreateGroupAPI)
		api.PUT("/cavi-groups/:id", authMiddleware.RequireAuth(), authMiddleware.RequireModerator(), h.UpdateGroupAPI)
		api.DELETE("/cavi-groups/:id", authMiddleware.RequireAuth(), authMiddleware.RequireModerator(), h.DeleteGroupAPI)
		api.POST("/cavi-groups/:id/image", authMiddleware.RequireAuth(), authMiddleware.RequireModerator(), h.UploadGroupImageAPI)
		api.POST("/cavi-groups/:id/add-to-draft", authMiddleware.RequireAuth(), h.AddGroupToDraftFromGroupAPI)

		api.GET("/cavi-calculations/draft", authMiddleware.RequireAuth(), h.GetCartIconAPI)
		api.GET("/cavi-calculations", authMiddleware.RequireAuth(), h.ListCalculationsAPI)
		api.GET("/cavi-calculations/:id", authMiddleware.RequireAuth(), h.GetCalculationAPI)
		api.PUT("/cavi-calculations/:id", authMiddleware.RequireAuth(), h.UpdateCalculationAPI)
		api.PUT("/cavi-calculations/draft/form", authMiddleware.RequireAuth(), h.FormCalculationAPI)
		api.PUT("/cavi-calculations/:id/moderate", authMiddleware.RequireAuth(), authMiddleware.RequireModerator(), h.ModerateCalculationAPI)
		api.DELETE("/cavi-calculations/:id", authMiddleware.RequireAuth(), h.DeleteCalculationAPI)

		api.DELETE("/cavi-calculations/draft/groups", authMiddleware.RequireAuth(), h.RemoveItemFromDraftAPI)
		api.PUT("/cavi-calculations/draft/groups", authMiddleware.RequireAuth(), h.UpdateItemInDraftAPI)
	}

	serverAddr := fmt.Sprintf("%s:%d", cfg.CaviServerHost, cfg.CaviServerPort)
	log.Printf("Server starting on %s", serverAddr)
	r.Run(serverAddr)
	log.Println("Server down")
}
