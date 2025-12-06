package handlers

import (
	"context"
	"encoding/json"
	"fmt"
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

	m.HandleFunc(prefix+"/get-thumb", enableCORS(func(w http.ResponseWriter, r *http.Request) {
		getThumb(w, r, minioClient)
	}))

	m.HandleFunc(prefix+"/get-variants", enableCORS(func(w http.ResponseWriter, r *http.Request) {
		getVariants(w, r, minioClient)
	}))
}

func getVideo(w http.ResponseWriter, r *http.Request, minioClient *services.MinioService) {
	videoPath := r.URL.Query().Get("jobId")
	if videoPath == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	url, err := presignIfExists(ctx, minioClient, videoPath)
	if err != nil {
		http.Error(w, "Failed to get presigned URL", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"url":"` + url + `"}`))
}

func getThumb(w http.ResponseWriter, r *http.Request, minioClient *services.MinioService) {
	thumbPath := r.URL.Query().Get("jobId")
	if thumbPath == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	url, err := presignIfExists(ctx, minioClient, thumbPath)
	if err != nil {
		http.Error(w, "Failed to get presigned URL", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"url":"` + url + `"}`))
}

func getVariants(w http.ResponseWriter, r *http.Request, minioClient *services.MinioService) {
	jobID := r.URL.Query().Get("jobId")
	if jobID == "" {
		http.Error(w, "jobId is required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	klingPath := fmt.Sprintf("shorts/%s/kling_video.mp4", jobID)
	imagePath := fmt.Sprintf("shorts/%s/image_video.mp4", jobID)
	thumbPath := fmt.Sprintf("shorts/%s/generate_image_0.jpg", jobID)

	klingURL, err := presignIfExists(ctx, minioClient, klingPath)
	if err != nil {
		http.Error(w, "Failed to sign kling video", http.StatusInternalServerError)
		return
	}

	imageURL, err := presignIfExists(ctx, minioClient, imagePath)
	if err != nil {
		http.Error(w, "Failed to sign image video", http.StatusInternalServerError)
		return
	}

	thumbURL, err := presignIfExists(ctx, minioClient, thumbPath)
	if err != nil {
		http.Error(w, "Failed to sign thumb", http.StatusInternalServerError)
		return
	}

	resp := struct {
		KlingURL string `json:"kling_url"`
		ImageURL string `json:"image_url"`
		ThumbURL string `json:"thumb_url"`
	}{
		KlingURL: klingURL,
		ImageURL: imageURL,
		ThumbURL: thumbURL,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonResp, _ := json.Marshal(resp)
	w.Write(jsonResp)
}

func presignIfExists(ctx context.Context, minioClient *services.MinioService, objectPath string) (string, error) {
	if objectPath == "" {
		return "", nil
	}

	const bucket = "shorts-maker"

	if _, err := minioClient.Client.StatObject(ctx, bucket, objectPath, minio.StatObjectOptions{}); err != nil {
		resp := minio.ToErrorResponse(err)
		if resp.StatusCode == http.StatusNotFound || resp.Code == "NoSuchKey" {
			return "", nil
		}
		log.Printf("StatObject failed for %s: %v", objectPath, err)
		return "", err
	}

	url, err := minioClient.GetPresignedURL(bucket, objectPath, 2*time.Hour)
	if err != nil {
		log.Printf("Failed to presign %s: %v", objectPath, err)
		return "", err
	}

	return url, nil
}
