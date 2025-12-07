package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/thedekerone/shorts-maker/handlers"
	"github.com/thedekerone/shorts-maker/services"
	"github.com/thedekerone/shorts-maker/services/store"
)

func main() {
	mux := http.NewServeMux()

	minioClient, err := services.ConnectToMinio()

	if err != nil {
		log.Fatal(err)
		return
	}

	time.Sleep(1 * time.Second)
	bucketExists, err := minioClient.Client.BucketExists(context.Background(), "shorts-maker")

	if err != nil {
		log.Fatal("failed to check if bucket exists:", err)
	}

	if !bucketExists {
		log.Print("bucket doesnt exist")
		err = minioClient.Client.MakeBucket(context.Background(), "shorts-maker", minio.MakeBucketOptions{})
		if err != nil {
			log.Fatal("failed to create bucket:", err)
		}

		// Set bucket policy to allow public read access
		policy := `{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Principal": "*",
					"Action": ["s3:GetObject"],
					"Resource": ["arn:aws:s3:::shorts-maker/*"]
				}
			]
		}`

		err = minioClient.Client.SetBucketPolicy(context.Background(), "shorts-maker", policy)
		if err != nil {
			log.Fatal("failed to set bucket policy:", err)
		}

		log.Println("Bucket created and set to public read access")
	}

	dbPath := os.Getenv("JOBS_DB_PATH")
	if dbPath == "" {
		dbPath = "jobs.db"
	}
	jobStore, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("failed to open job store: %v", err)
	}
	defer jobStore.Close()

	mux.HandleFunc("/ping", handlers.HealthCheckHandler)
	handlers.HandleReplicateRequest(mux, minioClient, jobStore)
	handlers.HandleVideoRequest(mux, minioClient)

	log.Fatal(http.ListenAndServe(":8080", mux))
}
