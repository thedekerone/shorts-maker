package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/pkg"
	"github.com/thedekerone/shorts-maker/services"
)

type Job struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	URL    string `json:"url"`
	Error  string `json:"error,omitempty"`
}

func (j Job) FormattedURL() string {
	return strings.ReplaceAll(j.URL, `\u0026`, "&")
}

var (
	jobs      = make(map[string]*Job)
	jobsMutex sync.RWMutex
)

func HandleReplicateRequest(m *http.ServeMux, minioClient *services.MinioService) {
	prefix := "/replicate"

	println("registering handlers")

	m.HandleFunc(prefix+"/generate-ai-short", enableCORS(generateAIShort))
	m.HandleFunc(prefix+"/job-status", enableCORS(getJobStatus))
	m.HandleFunc(prefix+"/test-sign-url", testSignURL)

	m.HandleFunc(prefix+"/get-completition", handleCompletition)
	m.HandleFunc(prefix+"/get-voice", handleGetVoice)
	m.HandleFunc(prefix+"/get-images", handleGetImages)
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

	predictions, err := rs.GetCompletition(prompt)

	print(predictions)

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

	object, err := minioClient.Client.PresignedGetObject(context.Background(), "shorts-maker", "shorts/test.mp4", time.Second*60*60*24, reqParams)
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

	println(voice)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(voice)
}

func handleGetImages(w http.ResponseWriter, r *http.Request) {
	rs, err := services.NewReplicateService()

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error creating replicate service"))
		return
	}

	prompt := r.URL.Query().Get("prompt")
	quantity := r.URL.Query().Get("quantity")

	if prompt == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("prompt is required"))
		return
	}

	if quantity == "" {
		quantity = "1"
	}

	s, err := strconv.ParseInt(quantity, 10, 8)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid quantity"))
		return
	}

	images, err := rs.GetImages(prompt, s)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error getting images"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(images)
}

func generateAIShort(w http.ResponseWriter, r *http.Request) {
	// Check if the request method is GET
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the text query parameter
	text := r.URL.Query().Get("text")

	// Validate the text parameter
	if text == "" {
		http.Error(w, "text parameter is required", http.StatusBadRequest)
		return
	}

	// Generate a unique job ID
	jobID := uuid.New().String()

	// Create a new job and store it in the jobs map
	job := &Job{
		ID:     jobID,
		Status: "initialized",
	}

	jobsMutex.Lock()
	jobs[jobID] = job
	jobsMutex.Unlock()

	// Start the video generation process in a goroutine
	go processVideoGeneration(jobID, text)

	// Prepare the response
	response := map[string]string{
		"jobId": jobID,
	}

	// Set the content type header
	w.Header().Set("Content-Type", "application/json")

	// Write the response
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

func processVideoGeneration(jobID string, text string) {

	updateJobStatus(jobID, "connecting_to_minio", "", "")
	minioClient, err := services.ConnectToMinio()
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Couldn't connect to minio: "+err.Error())
		return
	}

	updateJobStatus(jobID, "creating_replicate_service", "", "")
	rs, err := services.NewReplicateService()
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error creating replicate service: "+err.Error())
		return
	}

	updateJobStatus(jobID, "generating_script", "", "")
	predictions, err := rs.GetCompletition(text)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting completition: "+err.Error())
		return
	}

	updateJobStatus(jobID, "generating_voice", "", "")
	voice, err := rs.GetVoice(predictions)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting voice: "+err.Error())
		return
	}

	updateJobStatus(jobID, "generating_transcription", "", "")
	transcript, err := rs.GetTranscription(voice, predictions)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting transcription: "+err.Error())
		return
	}

	lastSegment := transcript.Segments[len(transcript.Segments)-1]

	updateJobStatus(jobID, "generating_images", "", "")
	images, err := getImagesWithTimestamps(transcript, predictions)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting images: "+err.Error())
		return
	}

	updateJobStatus(jobID, "creating_subtitle_file", "", "")
	subtitlesPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s.ass", pkg.GenerateRandomString(7)))
	err = pkg.CreateAssFile(subtitlesPath, *transcript)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error creating subtitle file: "+err.Error())
		return
	}

	updateJobStatus(jobID, "creating_video_from_images", "", "")
	path, err := pkg.MakeVideoOfImages(images, float32(lastSegment.End), os.TempDir())

	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error making video: "+err.Error())
		return
	}

	updateJobStatus(jobID, "adding_audio_to_video", "", "")
	outputPath, err := pkg.AddAudioToVideo(path, voice, subtitlesPath, os.TempDir())
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error adding audio to video: "+err.Error())
		return
	}

	updateJobStatus(jobID, "preparing_file_for_upload", "", "")
	file, err := os.Open(outputPath)
	if err != nil {
		print(err.Error())
		updateJobStatus(jobID, "failed", "", "Error opening file: "+err.Error())
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting file info: "+err.Error())
		return
	}
	fileSize := fileInfo.Size()
	fileExt := filepath.Ext(fileInfo.Name())
	generatedFileName := fmt.Sprintf("shorts/generated_short_%s%s", jobID, fileExt)

	updateJobStatus(jobID, "uploading_to_minio", "", "")
	_, err = minioClient.Client.PutObject(context.Background(), "shorts-maker", generatedFileName, file, fileSize, minio.PutObjectOptions{ContentType: "video/mp4"})
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error uploading file to Minio: "+err.Error())
		return
	}

	updateJobStatus(jobID, "generating_presigned_url", "", "")
	object, err := minioClient.Client.PresignedGetObject(context.Background(), "shorts-maker", generatedFileName, time.Hour*12, nil)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting presigned url: "+err.Error())
		return
	}

	// put only the part from just before the bucket name until the end
	videoSignedURL := object.String()[strings.Index(object.String(), "/shorts-maker"):]

	updateJobStatus(jobID, "completed", videoSignedURL, "")

	//only path
	// show only after the url

	// Clean up temporary files
	os.Remove(outputPath)
	os.Remove(path)
	os.Remove(subtitlesPath)
}

func getImagesWithTimestamps(transcript *models.TranscriptionOutput, script string) ([]models.ImageWithTimestamp, error) {
	rs, err := services.NewReplicateService()
	if err != nil {
		return nil, fmt.Errorf("error creating replicate service: %w", err)
	}

	totalDuration := transcript.Segments[len(transcript.Segments)-1].End
	interval := totalDuration / 4

	var imagesWithTimestamps []models.ImageWithTimestamp

	for i := 0; i < 4; i++ {
		timestamp := float64(i) * interval
		system := "I have the following story: \n" + script + "\n" + "Generate an image for this specific part"
		relevantText := getRelevantText(transcript, timestamp)

		// If relevantText is empty, use the text from the first segment
		if relevantText == "" && len(transcript.Segments) > 0 {
			relevantText = transcript.Segments[0].Text
		}

		images, err := rs.GetImages(system+relevantText, 1)
		if err != nil {
			return nil, fmt.Errorf("error getting image %d: %w", i+1, err)
		}

		if len(images) > 0 {
			imagesWithTimestamps = append(imagesWithTimestamps, models.ImageWithTimestamp{
				URL:       images[0],
				Timestamp: timestamp,
			})
		}
	}

	return imagesWithTimestamps, nil
}

func getRelevantText(transcript *models.TranscriptionOutput, timestamp float64) string {
	var relevantText string
	var currentSegmentIndex int

	// Find the current segment
	for i, segment := range transcript.Segments {
		if segment.Start <= timestamp && segment.End > timestamp {
			currentSegmentIndex = i
			break
		}
	}

	// Get text from the current segment to the start of the next segment (or end of transcript)
	for i := currentSegmentIndex; i < len(transcript.Segments); i++ {
		relevantText += transcript.Segments[i].Text + " "
		if i < len(transcript.Segments)-1 && transcript.Segments[i+1].Start > timestamp {
			break
		}
	}

	return strings.TrimSpace(relevantText)
}

func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next.ServeHTTP(w, r)
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

	// Create a new struct for the response
	response := struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		URL    string `json:"url"`
		Error  string `json:"error,omitempty"`
	}{
		ID:     job.ID,
		Status: job.Status,
		URL:    job.FormattedURL(),
		Error:  job.Error,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func updateJobStatus(jobID, status, url, errorMsg string) {
	jobsMutex.Lock()
	defer jobsMutex.Unlock()

	if job, exists := jobs[jobID]; exists {
		job.Status = status
		job.URL = url // Store the original URL
		job.Error = errorMsg
	}
}
