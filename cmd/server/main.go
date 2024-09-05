package main

import (
	"log"
	"net/http"

	"github.com/thedekerone/shorts-maker/handlers"
	"github.com/thedekerone/shorts-maker/services"
)

func main() {
	mux := http.NewServeMux()

	minioClient, err := services.ConnectToMinio()

	if err != nil {
		log.Fatal(err)
		return
	}

	mux.HandleFunc("/ping", handlers.HealthCheckHandler)
	handlers.HandleReplicateRequest(mux, minioClient)

	log.Fatal(http.ListenAndServe(":8080", mux))
}
