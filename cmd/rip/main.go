package main

import (
	"log"
	"rip/internal/api"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

// @title           CAVI Calculator API
// @version         1.0
// @description     API для расчёта индекса CAVI (Cardio-Ankle Vascular Index).
// @description     Система позволяет управлять группами пациентов, создавать заявки на расчёт,
// @description     добавлять группы в заявки и проводить модерацию.

// @contact.name   API Support
// @contact.email  support@cavi.local

// @host      localhost:8080
// @BasePath  /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT токен в формате: Bearer {token}

func main() {
	if err := godotenv.Load(); err != nil {
		logrus.Warn("No .env file found, using environment variables")
	}

	log.Println("CAVI Calculator starting!")
	api.StartServer()
	log.Println("CAVI Calculator terminated!")
}
