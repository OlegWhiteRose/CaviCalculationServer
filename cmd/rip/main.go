package main

import (
	"log"
	"rip/internal/api"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

// @title           CAVI Calculator API
// @version         1.0
// @description     API для управления заявками и группами CAVI с поддержкой аутентификации и авторизации
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@cavi.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8000
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Введите токен в формате: Bearer {token}

func main() {
	if err := godotenv.Load(); err != nil {
		logrus.Warn("No .env file found, using environment variables")
	}

	log.Println("CAVI Calculator starting!")
	api.StartServer()
	log.Println("CAVI Calculator terminated!")
}
