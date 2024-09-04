package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/thedekerone/shorts-maker/pkg"
	"github.com/thedekerone/shorts-maker/services"
)

func HandleReplicateRequest(m *http.ServeMux) {
	prefix := "/replicate"

	println("registering handlers")

	m.HandleFunc(prefix+"/test", test)
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

func test(w http.ResponseWriter, r *http.Request) {
	text := "write a scary story about a mansion in the woods"

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

	images, err := rs.GetImages(predictions, 4)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error getting images"))
		return
	}

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

	err = pkg.CreateAssFile(os.TempDir()+"testing.ass", *transcript)

	path, err := pkg.MakeVideoOfImages(images, 40, os.TempDir())

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

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("test"))
}
