package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thedekerone/shorts-maker/services"
)

func HandleReplicateRequest(m *http.ServeMux, minioClient *services.MinioService) {
	_ = minioClient // reserved for future use (upload helpers rely on global services)

	prefix := "/replicate"

	println("registering handlers")
	m.HandleFunc(prefix+"/generate-ai-short", enableCORS(generateAIShort))
	m.HandleFunc(prefix+"/generate-kling-short", enableCORS(generateKlingShort))
	m.HandleFunc(prefix+"/generate-image-short", enableCORS(generateImageShort))
	m.HandleFunc(prefix+"/job-status", enableCORS(getJobStatus))
	m.HandleFunc(prefix+"/test-sign-url", testSignURL)

	m.HandleFunc(prefix+"/get-completition", handleCompletition)
	m.HandleFunc(prefix+"/get-voice", handleGetVoice)
	m.HandleFunc(prefix, handleIndex)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/replicate" && r.URL.Path != "/replicate/" {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("replicate responded"))
}

func handleCompletition(w http.ResponseWriter, r *http.Request) {
	rs, err := services.NewReplicateService()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error creating replicate service"))
		return
	}

	prompt := r.URL.Query().Get("prompt")
	if prompt == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("prompt is required"))
		return
	}

	predictions, err := rs.GetCompletition(prompt, "")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error getting completion"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(predictions))
}

func testSignURL(w http.ResponseWriter, r *http.Request) {
	minioClient, err := services.ConnectToMinio()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error connecting to minio"))
		return
	}

	reqParams := make(url.Values)
	reqParams.Set("response-content-disposition", "attachment; filename=\"test.mp4\"")

	object, err := minioClient.Client.PresignedGetObject(context.Background(), "shorts-maker", "shorts/test.mp4", time.Hour*24, reqParams)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error getting presigned url"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(object.String()))
}

func handleGetVoice(w http.ResponseWriter, r *http.Request) {
	rs, err := services.NewReplicateService()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error creating replicate service"))
		return
	}

	prompt := r.URL.Query().Get("prompt")
	if prompt == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("prompt is required"))
		return
	}

	voice, err := rs.GetVoice(prompt)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error getting voice"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(voice)
}

func generateAIShort(w http.ResponseWriter, r *http.Request) {
	handleGenerateShort(w, r, generationOptions{GenerateKling: true, GenerateImages: true})
}

func generateKlingShort(w http.ResponseWriter, r *http.Request) {
	handleGenerateShort(w, r, generationOptions{GenerateKling: true})
}

func generateImageShort(w http.ResponseWriter, r *http.Request) {
	handleGenerateShort(w, r, generationOptions{GenerateImages: true})
}

func handleGenerateShort(w http.ResponseWriter, r *http.Request, opts generationOptions) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !opts.GenerateKling && !opts.GenerateImages {
		http.Error(w, "no generation variant selected", http.StatusBadRequest)
		return
	}

	req, err := parseGenerationRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jobID, err := enqueueGenerationJob(req, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJobAcceptedResponse(w, jobID)
}

func parseGenerationRequest(r *http.Request) (generationRequest, error) {
	var req generationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, fmt.Errorf("invalid request body: %w", err)
	}

	cleanScript := strings.ReplaceAll(req.Script, "\n", " ")
	req.Script = strings.TrimSpace(cleanScript)
	if req.Script == "" {
		return req, errors.New("script parameter is required")
	}

	req.Webhook = resolveWebhookURL(req.Webhook)
	req.Mode = normalizeMode(req.Mode)
	req.VisualStyle = strings.TrimSpace(req.VisualStyle)
	return req, nil
}

func enqueueGenerationJob(req generationRequest, opts generationOptions) (string, error) {
	if !opts.GenerateKling && !opts.GenerateImages {
		return "", errors.New("no generation variant selected")
	}

	jobID := uuid.New().String()
	job := &Job{ID: jobID, Status: "initialized"}

	jobsMutex.Lock()
	jobs[jobID] = job
	jobsMutex.Unlock()

	go processVideoGeneration(jobID, req.Script, req.Webhook, req.VoiceID, req.Mode, req.VisualStyle, opts)

	return jobID, nil
}

func writeJobAcceptedResponse(w http.ResponseWriter, jobID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(map[string]string{"jobId": jobID}); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

func getJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("jobId")
	if jobID == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("jobId is required"))
		return
	}

	jobsMutex.RLock()
	job, exists := jobs[jobID]
	jobsMutex.RUnlock()

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Job not found"))
		return
	}

	response := struct {
		ID        string `json:"id"`
		Status    string `json:"status"`
		URL       string `json:"url"`
		KlingURL  string `json:"kling_url,omitempty"`
		ImagesURL string `json:"images_url,omitempty"`
		Error     string `json:"error,omitempty"`
	}{
		ID:        job.ID,
		Status:    job.Status,
		URL:       job.FormattedURL(),
		KlingURL:  cleanURL(job.KlingURL),
		ImagesURL: cleanURL(job.ImagesURL),
		Error:     job.Error,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	}
}
