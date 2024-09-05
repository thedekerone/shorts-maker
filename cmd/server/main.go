package main

import (
	"context"
	"log"
	"net/http"

	"github.com/minio/minio-go/v7"
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

	bucketExists, err := minioClient.Client.BucketExists(context.Background(), "shorts-maker")

	if err != nil {
		log.Fatal("failed to check if bucket exists:", err)
	}

	if !bucketExists {
		err = minioClient.Client.MakeBucket(context.Background(), "shorts-maker", minio.MakeBucketOptions{})
		if err != nil {
			log.Fatal("failed to create bucket:", err)
		}
	}

	mux.HandleFunc("/ping", handlers.HealthCheckHandler)
	handlers.HandleReplicateRequest(mux, minioClient)

	log.Fatal(http.ListenAndServe(":8080", mux))
}
