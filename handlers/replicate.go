package handlers

import (
	"net/http"

	"github.com/thedekerone/shorts-maker/services"
)

func HandleReplicateRequest(m *http.ServeMux) {
	prefix := "/replicate"

	println("registering handlers")

	m.HandleFunc(prefix+"/get-completition", handleCompletition)
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

	print(predictions.Script)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(predictions.Script))

}
