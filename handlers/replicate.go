package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/replicate/replicate-go"
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
	ctx := context.TODO()

	prompt := r.URL.Query().Get("prompt")

	if prompt == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("prompt is required"))
		return
	}

	model := "meta/meta-llama-3-70b-instruct:fbfb20b472b2f3bdd101412a9f70a0ed4fc0ced78a77ff00970ee7a2383c575d"
	r8, err := replicate.NewClient(replicate.WithTokenFromEnv())

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error creating client"))
		return
	}

	input := replicate.PredictionInput{
		"system_prompt": "You are a tiktok script writer, you need to generate interesting stories for your audience. respond in the json format: {script: string; tags: string[]}. return only the json, don't include any other text.",
		"prompt":        prompt,
	}

	output, err := r8.Run(ctx, model, input, nil)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error running model"))
		return
	}

	if output == nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("output is nil"))
		return
	}

	stringOutput := outputToString(output)
	test := PredictionResponse{}

	json.Unmarshal([]byte(stringOutput), &test)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(stringOutput))

}

type PredictionResponse struct {
	Script string   `json:"script"`
	Tags   []string `json:"tags"`
}

func outputToString(output replicate.PredictionOutput) string {
	stringOutput := make([]string, len(output.([]any)))

	for i, v := range output.([]any) {
		str, ok := v.(string)

		if !ok {
			return ""
		}
		stringOutput[i] = str
	}

	return strings.Join(stringOutput, "")
}
