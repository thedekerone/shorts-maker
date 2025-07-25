package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/thedekerone/shorts-maker/app/images"
	"github.com/thedekerone/shorts-maker/elevenlabs"
	"github.com/thedekerone/shorts-maker/engine"
	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/pkg"
	"github.com/thedekerone/shorts-maker/services"
	"github.com/thedekerone/shorts-maker/subtitles"
)

func generateUniqueName() string {
	timestamp := time.Now().UnixNano()
	uuid := uuid.New().String()
	return fmt.Sprintf("%d_%s", timestamp, uuid)
}

type Job struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	URL    string `json:"url"`
	Error  string `json:"error,omitempty"`
}

type CompletedWebhookBody struct {
	URL string `json:"url"`
	ID  string `json:"id"`
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

func generateAIShort(w http.ResponseWriter, r *http.Request) {
	// Check if the request method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var requestBody struct {
		Script       string `json:"script"`
		Webhook      string `json:"webhook"`
		CaptionStyle string `json:"caption_style"`
		VoiceId      string `json:"voice_id"`
		Mode         string `json:"mode"`
		MusicId      string `json:"music_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the script parameter
	if requestBody.Script == "" {
		http.Error(w, "script parameter is required", http.StatusBadRequest)
		return
	}

	script := requestBody.Script
	webhook := "http://localhost:3000" + requestBody.Webhook

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
	go processVideoGeneration(jobID, script, webhook, requestBody.VoiceId, requestBody.Mode)

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

func uploadGeneratedFile(mio *services.MinioService, filePath string, fileName string, jobID string) error {
	file, err := os.Open(filePath)

	if err != nil {
		print(err.Error())
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting file info: "+err.Error())
		return err
	}
	fileSize := fileInfo.Size()
	fileExt := filepath.Ext(fileInfo.Name())
	generatedFileName := fmt.Sprintf("shorts/%s/%s%s", jobID, fileName, fileExt)

	_, err = mio.Client.PutObject(context.Background(), "shorts-maker", generatedFileName, file, fileSize, minio.PutObjectOptions{ContentType: "video/mp4"})

	return nil
}

func processVideoGeneration(jobID string, script string, webhook string, voiceId string, mode string) {
	print("dasdasads")

	minioClient, err := connectToMinio(jobID)
	if err != nil {
		return
	}

	rs, err := createReplicateService(jobID)
	if err != nil {
		return
	}

	voice, transcript, err := generateVoice(jobID, script, voiceId)
	if err != nil {
		return
	}

	err = uploadGeneratedFile(minioClient, voice, "voice_script", jobID)
	if err != nil {
		println("Failed to upload voice to minio")
		return
	}

	if transcript == nil {
		transcript, err = generateTranscription(jobID, rs, voice, script)
		if err != nil {
			return
		}
	}

	images, err := generateImages(jobID, transcript, mode)
	if err != nil {
		return
	}

	for i, v := range images {
		err = uploadGeneratedFile(minioClient, v.URL, fmt.Sprintf("generate_image_%d", i), jobID)
		if err != nil {
			println("Failed to upload image to minio")
			return
		}

	}

	outputFilePath, err := createVideo(jobID, transcript, images, voice, mode)
	if err != nil {
		return
	}

	err = uploadToMinio(jobID, minioClient, outputFilePath)
	if err != nil {
		return
	}

	whBody := CompletedWebhookBody{
		ID:  jobID,
		URL: outputFilePath,
	}

	jsonBody, err := json.Marshal(&whBody)
	if err != nil {
		return
	}
	//call webhook
	_, err = http.NewRequest("POST", webhook, bytes.NewBuffer(jsonBody))
	if err != nil {
		return
	}

	// Clean up temporary files
	os.Remove(outputFilePath)
}

func connectToMinio(jobID string) (*services.MinioService, error) {
	updateJobStatus(jobID, "connecting_to_minio", "", "")
	minioClient, err := services.ConnectToMinio()
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Couldn't connect to minio: "+err.Error())
		return nil, err
	}
	return minioClient, nil
}

func createReplicateService(jobID string) (*services.ReplicateService, error) {
	updateJobStatus(jobID, "creating_replicate_service", "", "")
	rs, err := services.NewReplicateService()
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error creating replicate service: "+err.Error())
		return nil, err
	}
	return rs, nil
}

func generateVoice(jobID string, predictions string, voiceid string) (string, *models.TranscriptionOutput, error) {
	updateJobStatus(jobID, "generating_voice", "", "")
	n := elevenlabs.CreateEleven()
	if voiceid == "" {
		voiceid = "NNl6r8mD7vthiJatiJt1"
	}

	vr := n.NewVoiceRequestMultilingual(predictions, voiceid)

	audio, err := vr.Call(os.TempDir()+pkg.GenerateRandomString(6)+".mp3", true)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting audio: "+err.Error())
		return "", nil, err
	}

	return audio.AudioPath, audio.Transcription, nil
}

func generateTranscription(jobID string, rs *services.ReplicateService, voice string, predictions string) (*models.TranscriptionOutput, error) {
	updateJobStatus(jobID, "generating_transcription", "", "")
	transcript, err := rs.GetTranscription(voice, predictions)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting transcription: "+err.Error())
		return nil, err
	}
	return transcript, nil
}

func generateImages(jobID string, transcript *models.TranscriptionOutput, mode string) ([]models.ImageWithTimestamp, error) {
	updateJobStatus(jobID, "generating_images", "", "")
	images, err := images.GetImagesWithTimestamps(transcript, mode)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting images: "+err.Error())
		return nil, err
	}
	return images, nil
}

func createVideo(jobID string, transcript *models.TranscriptionOutput, images []models.ImageWithTimestamp, voice string, mode string) (string, error) {
	ctx := context.Background()
	updateJobStatus(jobID, "creating_subtitle_file", "", "")

	updateJobStatus(jobID, "creating_video_from_images", "", "")
	totalDuration := transcript.Segments[len(transcript.Segments)-1].End

	path, err := engine.CreateVideoFromImages(images, os.TempDir()+pkg.GenerateRandomString(6)+".mp4", totalDuration, engine.TransitionTypeFade, mode)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error making video: "+err.Error())
		return "", err
	}

	updateJobStatus(jobID, "adding_audio_to_video", "", "")
	outputPath, err := pkg.AddAudioToVideo(path.Path, voice, os.TempDir())
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error adding audio to video: "+err.Error())
		return "", err
	}

	outputFileName := fmt.Sprintf("%s.mp4", generateUniqueName())
	outputFilePath := filepath.Join(os.TempDir(), outputFileName)
	subStyles := subtitles.SubtitleStyles{
		FontFamily:  "Roboto-Black",
		FontSize:    72,
		BorderColor: "black",
		BorderWidth: 4,
		Color:       "white",
	}
	animationSubs := subtitles.CreateShortSubsWithStyles(transcript, &subStyles)

	updateJobStatus(jobID, "generating ASS file", "", "")
	fmt.Printf("%v", err)
	baseAssPath, err := filepath.Abs("handlers/assets/base.ass")
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting absolute path for base.ass: "+err.Error())
		return "", err
	}
	subtitlesPath, err := subtitles.CreateAssFile(animationSubs, baseAssPath)

	fmt.Printf(subtitlesPath)

	fmt.Printf("%v", err)
	if err != nil {
		return "", err
	}
	updateJobStatus(jobID, "Adding subtitles to video", "", "")

	_, err = engine.AddAssSubtitlesToVideo(ctx, outputPath, subtitlesPath, outputFilePath)

	if err != nil {
		return "", err

	}

	return outputFilePath, nil
}

func uploadToMinio(jobID string, minioClient *services.MinioService, outputFilePath string) error {
	updateJobStatus(jobID, "preparing_file_for_upload", "", "")
	file, err := os.Open(outputFilePath)
	if err != nil {
		print(err.Error())
		updateJobStatus(jobID, "failed", "", "Error opening file: "+err.Error())
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting file info: "+err.Error())
		return err
	}
	fileSize := fileInfo.Size()
	fileExt := filepath.Ext(fileInfo.Name())
	generatedFileName := fmt.Sprintf("shorts/%s/generated_short_%s", jobID, fileExt)

	updateJobStatus(jobID, "uploading_to_minio", "", "")
	_, err = minioClient.Client.PutObject(context.Background(), "shorts-maker", generatedFileName, file, fileSize, minio.PutObjectOptions{ContentType: "video/mp4"})
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error uploading file to Minio: "+err.Error())
		return err
	}

	updateJobStatus(jobID, "generating_presigned_url", "", "")
	object, err := minioClient.Client.PresignedGetObject(context.Background(), "shorts-maker", generatedFileName, time.Hour*12, nil)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting presigned url: "+err.Error())
		return err
	}

	println(object.String())
	// put only the part from just before the bucket name until the end
	videoSignedURL := object.String()[strings.Index(object.String(), "/shorts-maker"):]

	updateJobStatus(jobID, "completed", videoSignedURL, "")

	return nil
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
