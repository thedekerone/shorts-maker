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
	// Check if the request method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var requestBody struct {
		Script string `json:"script"`
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
	go processVideoGeneration(jobID, script)

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

func processVideoGeneration(jobID string, script string) {
	print("dasdasads")

	minioClient, err := connectToMinio(jobID)
	if err != nil {
		return
	}

	rs, err := createReplicateService(jobID)
	if err != nil {
		return
	}

	voice, err := generateVoice(jobID, rs, script)
	if err != nil {
		return
	}

	err = uploadGeneratedFile(minioClient, voice, "voice_script", jobID)
	if err != nil {
		println("Failed to upload voice to minio")
		return
	}

	transcript, err := generateTranscription(jobID, rs, voice, script)
	if err != nil {
		return
	}

	images, err := generateImages(jobID, transcript, script)
	if err != nil {
		return
	}

	for i, v := range images {
		uploadGeneratedFile(minioClient, v.URL, fmt.Sprintf("generate_image_%d", i), jobID)
		if err != nil {
			println("Failed to upload image to minio")
			return
		}

	}

	outputFilePath, err := createVideo(jobID, transcript, images, voice)
	if err != nil {
		return
	}

	err = uploadToMinio(jobID, minioClient, outputFilePath)
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

func generateScript(jobID string, rs *services.ReplicateService, text string, script string) (string, error) {
	updateJobStatus(jobID, "generating_script", "", "")

	var predictions string
	var err error

	if script == "" {
		predictions, err = rs.GetCompletition(text, "")
	} else {
		predictions = script
	}
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting completition: "+err.Error())
		return "", err
	}
	return predictions, nil
}

func generateVoice(jobID string, rs *services.ReplicateService, predictions string) (string, error) {
	updateJobStatus(jobID, "generating_voice", "", "")
	n := elevenlabs.CreateEleven()

	vr := n.NewVoiceRequest(predictions, "pqHfZKP75CvOlQylNhV4")

	audioPath, err := vr.Call(os.TempDir() + pkg.GenerateRandomString(6) + ".mp3")
	println("generating audiooooooooooooo!!!")
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting audio: "+err.Error())
		println("error generating audioooooooo!!!")
		fmt.Println("%v", err)
		return "", err
	}

	println("finished generating audioooooooo!!!")

	return audioPath, nil
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

func generateImages(jobID string, transcript *models.TranscriptionOutput, predictions string) ([]models.ImageWithTimestamp, error) {
	updateJobStatus(jobID, "generating_images", "", "")
	images, err := getImagesWithTimestamps(transcript, predictions, 12)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting images: "+err.Error())
		return nil, err
	}
	return images, nil
}

func createVideo(jobID string, transcript *models.TranscriptionOutput, images []models.ImageWithTimestamp, voice string) (string, error) {
	ctx := context.Background()
	updateJobStatus(jobID, "creating_subtitle_file", "", "")

	updateJobStatus(jobID, "creating_video_from_images", "", "")
	totalDuration := transcript.Segments[len(transcript.Segments)-1].End

	path, err := engine.CreateVideoFromImages(images, os.TempDir()+pkg.GenerateRandomString(6)+".mp4", totalDuration)
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

	if err != nil {
		return "", err
	}

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

func getImagesWithTimestamps(transcript *models.TranscriptionOutput, script string, numImages int32) ([]models.ImageWithTimestamp, error) {
	rs, err := services.NewReplicateService()
	if err != nil {
		return nil, fmt.Errorf("error creating replicate service: %w", err)
	}

	totalDuration := transcript.Segments[len(transcript.Segments)-1].End
	interval := totalDuration / float64(numImages)

	var imagesWithTimestamps []models.ImageWithTimestamp

	imageGenerationPrompts := fmt.Sprintf(`You are a image prompt generator, the images generated should be interesting and with the tone of the story they should evolve taking in consideration the story, describe the images prompts well and don't forget to mention the styles of the image in the prompt, the user will provide a story divided in segments with timestamps and you have to return a JSON with the following format, JUST WRITE THE JSON, AVOID WRITING EXTRA TEXT. Describe the image style and camera settings, keep the styles consistant between images, numImages should depend on the length of the story. the sum of all the images duration should be %.2f:
		{
			numImages: number,
			images: [
				{ prompt: "string", segment: "part of the story where the image shows", duration: 12.0 }
			]
		}

		`, totalDuration)

	println("1222222222222222222222222222222222222")
	println(imageGenerationPrompts)

	var segmentStrings string

	for _, v := range transcript.Segments {
		segmentStrings = segmentStrings + fmt.Sprintf(" segment: %s ----- start: %.3f ----- end: %.3f \n", v.Text, v.Start, v.End)
	}

	promptForImage, err := rs.
		GetCompletitionForImages(imageGenerationPrompts+fmt.Sprintf("%s \n %v", script, segmentStrings), "")

	println("%v", promptForImage)
	for i := 0; i < int(promptForImage.NumImages); i++ {
		timestamp := float64(i) * interval

		relevantText := getRelevantText(transcript, timestamp)

		// If relevantText is empty, use the text from the first segment
		if relevantText == "" && len(transcript.Segments) > 0 {
			relevantText = transcript.Segments[0].Text
		}

		images, err := rs.GetImages(promptForImage.ImagesPrompt[i].Prompt, 1)
		if err != nil {
			return nil, fmt.Errorf("error getting image %d: %w", i+1, err)
		}

		if len(images) > 0 {
			imagesWithTimestamps = append(imagesWithTimestamps, models.ImageWithTimestamp{
				URL:       images[0],
				Timestamp: promptForImage.ImagesPrompt[i].Duration,
			})
		}
	}

	fmt.Println("00000000000000000000000000000000000000000000000")
	fmt.Println("%v", imagesWithTimestamps)

	return imagesWithTimestamps, nil
}

func findTimestampForImage(script string, transcript *models.TranscriptionOutput, segmentToFind string) float64 {
	result := strings.Split(script, segmentToFind)
	wordCount := len(strings.Fields(result[0]))

	var allWords []models.Word
	for _, v := range transcript.Segments {
		allWords = append(allWords, v.Words...)
	}
	println("=========================================")
	println("=========================================")
	println("=========================================")
	fmt.Println("%v", result)
	fmt.Println("%v", allWords)

	if len(allWords)-1 <= wordCount {
		wordCount = len(allWords) - 1
	}

	return allWords[wordCount].Start
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
