package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/pkg"
	"github.com/thedekerone/shorts-maker/services"
)

func HandleReplicateRequest(m *http.ServeMux, minioClient *services.MinioService) {
	prefix := "/replicate"

	println("registering handlers")

	m.HandleFunc(prefix+"/generate-ai-short", generateAIShort)
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
	text := r.URL.Query().Get("text")

	if text == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("text is required"))
		return
	}

	minioClient, err := services.ConnectToMinio()

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("couldn't connect to minio"))
	}

	rs, err := services.NewReplicateService()

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error creating replicate service"))
		return
	}

	predictions, err := rs.GetCompletition(text)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error getting completition"))
		return
	}

	println(predictions)

	voice, err := rs.GetVoice(predictions)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error getting voice"))
		return
	}

	transcript, err := rs.GetTranscription(voice, predictions)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error getting transcription"))
		fmt.Print(err.Error())
		return
	}

	lastSegment := transcript.Segments[len(transcript.Segments)-1]

	images, err := getImagesWithTimestamps(transcript)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error getting images"))
		return
	}

	err = pkg.CreateAssFile(os.TempDir()+"testing.ass", *transcript)

	path, err := pkg.MakeVideoOfImages(images, int(lastSegment.End), os.TempDir())

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error making video"))
		return
	}

	outputPath, err := pkg.AddAudioToVideo(path, voice, os.TempDir())

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error adding audio to video"))
		return
	}

	println(outputPath)

	file, err := os.Open(outputPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error opening file"))
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error getting file info"))
		return
	}
	fileSize := fileInfo.Size()
	fileExt := filepath.Ext(fileInfo.Name())
	generatedFileName := fmt.Sprintf("shorts/generated_short_%d%s", time.Now().Unix(), fileExt)

	_, err = minioClient.Client.PutObject(context.Background(), "shorts-maker", generatedFileName, file, fileSize, minio.PutObjectOptions{ContentType: "video/mp4"})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error uploading file to Minio"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("File uploaded successfully: %s", generatedFileName)))
}

func getImagesWithTimestamps(transcript *models.TranscriptionOutput) ([]models.ImageWithTimestamp, error) {
	rs, err := services.NewReplicateService()
	if err != nil {
		return nil, fmt.Errorf("error creating replicate service: %w", err)
	}

	totalDuration := transcript.Segments[len(transcript.Segments)-1].End
	interval := totalDuration / 4

	var imagesWithTimestamps []models.ImageWithTimestamp

	for i := 0; i < 4; i++ {
		timestamp := float64(i) * interval
		relevantText := getRelevantText(transcript, timestamp)

		// If relevantText is empty, use the text from the first segment
		if relevantText == "" && len(transcript.Segments) > 0 {
			relevantText = transcript.Segments[0].Text
		}

		images, err := rs.GetImages(relevantText, 1)
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
