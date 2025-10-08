package main

import (
	"log"

	"rip/internal/api"
)

func main() {
	log.Println("CAVI Calculator starting!")
	api.StartServer()
	log.Println("CAVI Calculator terminated!")
}
