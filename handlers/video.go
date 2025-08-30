package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/thedekerone/shorts-maker/services"
)

func HandleVideoRequest(m *http.ServeMux, minioClient *services.MinioService) {
	prefix := "/video"

	m.HandleFunc(prefix+"/get-video", enableCORS(func(w http.ResponseWriter, r *http.Request) {
		getVideo(w, r, minioClient)
	}))

	// NEW: /video/get-thumb -> returns a presigned URL for the thumbnail
	m.HandleFunc(prefix+"/get-thumb", enableCORS(func(w http.ResponseWriter, r *http.Request) {
		getThumb(w, r, minioClient)
	}))
}

func getVideo(w http.ResponseWriter, r *http.Request, minioClient *services.MinioService) {
	videoPath := r.URL.Query().Get("jobId")
	if videoPath == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	const bucket = "shorts-maker"
	ctx := context.Background()

	if _, err := minioClient.Client.StatObject(ctx, bucket, videoPath, minio.StatObjectOptions{}); err != nil {
		resp := minio.ToErrorResponse(err)
		if resp.StatusCode == http.StatusNotFound || resp.Code == "NoSuchKey" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"url":undefined}`))
			return
		}
		log.Printf("StatObject failed: %v", err)
		http.Error(w, "Failed to check object", http.StatusInternalServerError)
		return
	}

	presignedURL, err := minioClient.GetPresignedURL(bucket, videoPath, 2*time.Hour)
	if err != nil {
		log.Printf("Failed to get presigned URL: %v", err)
		http.Error(w, "Failed to get presigned URL", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"url":"` + presignedURL + `"}`))
}

// NEW: generates a presigned URL for the thumbnail in the same bucket
func getThumb(w http.ResponseWriter, r *http.Request, minioClient *services.MinioService) {
	thumbPath := r.URL.Query().Get("jobId")
	if thumbPath == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	const bucket = "shorts-maker"
	ctx := context.Background()

	// Ensure the object exists
	if _, err := minioClient.Client.StatObject(ctx, bucket, thumbPath, minio.StatObjectOptions{}); err != nil {
		resp := minio.ToErrorResponse(err)
		if resp.StatusCode == http.StatusNotFound || resp.Code == "NoSuchKey" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Mirrors your get-video behavior so the frontend's try/catch continues to work
			w.Write([]byte(`{"url":undefined}`))
			return
		}
		log.Printf("StatObject failed (thumb): %v", err)
		http.Error(w, "Failed to check object", http.StatusInternalServerError)
		return
	}

	presignedURL, err := minioClient.GetPresignedURL(bucket, thumbPath, 2*time.Hour)
	if err != nil {
		log.Printf("Failed to get presigned URL (thumb): %v", err)
		http.Error(w, "Failed to get presigned URL", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"url":"` + presignedURL + `"}`))
}
