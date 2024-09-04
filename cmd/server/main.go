package main

import (
	"log"
	"net/http"

	"github.com/thedekerone/shorts-maker/handlers"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", handlers.HealthCheckHandler)
	handlers.HandleReplicateRequest(mux)

	log.Fatal(http.ListenAndServe(":8080", mux))
}
