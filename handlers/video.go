package handlers

import (
	"github.com/thedekerone/shorts-maker/services"
	"log"
	"net/http"
)

func HandleVideoRequest(m *http.ServeMux, minioClient *services.MinioService) {
	prefix := "/video"

	m.HandleFunc(prefix+"/get-video", enableCORS(func(w http.ResponseWriter, r *http.Request) {
		getVideo(w, r, minioClient)
	}))
}

func getVideo(w http.ResponseWriter, r *http.Request, minioClient *services.MinioService) {
	videoPath := r.URL.Query().Get("path")
	if videoPath == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	presignedURL, err := minioClient.GetPresignedURL("shorts-maker", videoPath, 60*60)
	if err != nil {
		log.Printf("Failed to get presigned URL: %v", err)
		http.Error(w, "Failed to get presigned URL", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"url":"` + presignedURL + `"}`))
}
