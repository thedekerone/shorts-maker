package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/thedekerone/shorts-maker/pkg"
	"github.com/thedekerone/shorts-maker/services"
)

func HandleReplicateRequest(m *http.ServeMux, minioClient *services.MinioService) {
	prefix := "/replicate"

	println("registering handlers")

	m.HandleFunc(prefix+"/generate-ai-short", generateAIShort)
	m.HandleFunc(prefix+"/get-completition", handleCompletition)
	m.HandleFunc(prefix+"/get-voice", handleGetVoice)
	m.HandleFunc(prefix+"/get-transcription", handleGetTranscription)
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

func handleGetTranscription(w http.ResponseWriter, r *http.Request) {
	rs, err := services.NewReplicateService()

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error creating replicate service"))
		return
	}

	transcript, err := rs.GetTranscription("https://replicate.delivery/yhqm/0GnXQ1h0wZaEEdCw9NDyTNTsYV5Px4MZUDVef8e9fiU6eXMbC/output.wav", "")

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error getting transcription"))
		fmt.Print(err.Error())
		return
	}

	print(os.TempDir() + "testing.ass")

	err = pkg.CreateAssFile(os.TempDir()+"testing.ass", *transcript)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(transcript)
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

	println(voice)

	transcript, err := rs.GetTranscription(voice, predictions)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error getting transcription"))
		fmt.Print(err.Error())
		return
	}

	lastSegment := transcript.Segments[len(transcript.Segments)-1]

	images, err := rs.GetImages(predictions, 4)

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

	audioPath := voice

	outputPath, err := pkg.AddAudioToVideo(path, audioPath, os.TempDir())

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error adding audio to video"))
		return
	}

	println("finished")
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
	generatedFileName := fmt.Sprintf("generated_short_%d.%s", time.Now().Unix(), fileExt)

	_, err = minioClient.Client.PutObject(context.Background(), "shorts-maker", generatedFileName, file, fileSize, minio.PutObjectOptions{ContentType: "video/mp4"})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error uploading file to Minio"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("File uploaded successfully: %s", generatedFileName)))
}
