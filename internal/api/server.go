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

	// Инициализация Redis
	redis, err := redisClient.NewRedisClient()
	if err != nil {
		logrus.Fatal("Failed to initialize Redis:", err)
	}
	log.Println("Redis connected successfully")

	// Инициализация handler с Redis
	h := handler.NewHandler(cfg, repo, minioStorage, redis)

	// Инициализация auth middleware
	authMiddleware := middleware.NewAuthMiddleware(redis)

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./resources")

	// HTML роуты (без авторизации для просмотра)
	r.GET("/", h.GetCaviGroups)
	r.GET("/cavi-group/:id", h.GetCaviGroup)
	r.GET("/calculations/:id", h.GetCaviCalculationByID)
	
	// HTML роуты требующие авторизации
	r.POST("/add-group", h.AddGroupToCalculation)
	r.POST("/calculations/:id/delete", h.SoftDeleteCalculationByID)

	// Swagger UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")
	{
		// Аутентификация (публичные роуты)
		auth := api.Group("/auth")
		{
			auth.POST("/register", h.Register)
			auth.POST("/login", h.Login)
			auth.POST("/logout", authMiddleware.RequireAuth(), h.Logout)
			auth.GET("/me", authMiddleware.RequireAuth(), h.GetCurrentUser)
		}

		// Группы CAVI (гость - только GET, пользователь - GET, модератор - все)
		groups := api.Group("/cavi-groups")
		{
			// Публичные (гость)
			groups.GET("", h.GetGroupsAPI)
			groups.GET("/:id", h.GetGroupAPI)
			
			// Только для модератора
			groups.POST("", authMiddleware.RequireAuth(), authMiddleware.RequireModerator(), h.CreateGroupAPI)
			groups.PUT("/:id", authMiddleware.RequireAuth(), authMiddleware.RequireModerator(), h.UpdateGroupAPI)
			groups.DELETE("/:id", authMiddleware.RequireAuth(), authMiddleware.RequireModerator(), h.DeleteGroupAPI)
			groups.POST("/:id/image", authMiddleware.RequireAuth(), authMiddleware.RequireModerator(), h.UploadGroupImageAPI)
			
			// Для авторизованных пользователей
			groups.POST("/:id/add-to-draft", authMiddleware.RequireAuth(), h.AddGroupToDraftFromGroupAPI)
		}

		// Заявки CAVI
		calculations := api.Group("/cavi-calculations")
		{
			// Для авторизованных пользователей
			calculations.GET("/draft", authMiddleware.RequireAuth(), h.GetCartIconAPI)
			calculations.GET("", authMiddleware.RequireAuth(), h.ListCalculationsAPI)
			calculations.GET("/:id", authMiddleware.RequireAuth(), h.GetCalculationAPI)
			calculations.PUT("/:id", authMiddleware.RequireAuth(), h.UpdateCalculationAPI)
			calculations.PUT("/:id/form", authMiddleware.RequireAuth(), h.FormCalculationAPI)
			calculations.DELETE("/:id", authMiddleware.RequireAuth(), h.DeleteCalculationAPI)
			
			// Только для модератора
			calculations.PUT("/:id/moderate", authMiddleware.RequireAuth(), authMiddleware.RequireModerator(), h.ModerateCalculationAPI)
			
			// Работа с черновиком
			calculations.DELETE("/draft/groups", authMiddleware.RequireAuth(), h.RemoveItemFromDraftAPI)
			calculations.PUT("/draft/groups", authMiddleware.RequireAuth(), h.UpdateItemInDraftAPI)
		}
	}

	serverAddr := fmt.Sprintf("%s:%d", cfg.CaviServerHost, cfg.CaviServerPort)
	log.Printf("Server starting on %s", serverAddr)
	r.Run(serverAddr)
	log.Println("Server down")
}
