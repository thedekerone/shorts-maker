package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIService_GenerateImagePlan(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		resp := openAIResponse{
			Choices: []struct {
				Message openAIMessage `json:"message"`
			}{
				{Message: openAIMessage{Content: `{"numImages":1,"stylePrompt":"STYLE","images":[{"prompt":"scene","duration":5}]}`}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(ts.Close)

	svc := &OpenAIService{
		apiKey:     "test",
		baseURL:    ts.URL,
		model:      "gpt-test",
		httpClient: ts.Client(),
	}

	plan, err := svc.GenerateImagePlan("user", "system")
	if err != nil {
		t.Fatalf("GenerateImagePlan error: %v", err)
	}
	if plan.NumImages != 1 {
		t.Fatalf("unexpected num images: %d", plan.NumImages)
	}
}

func TestOpenAIService_GenerateClipPlan_ErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(ts.Close)

	svc := &OpenAIService{
		apiKey:     "test",
		baseURL:    ts.URL,
		model:      "gpt-test",
		httpClient: ts.Client(),
	}

	if _, err := svc.GenerateClipPlan("user", "system"); err == nil {
		t.Fatal("expected error for non-200 response")
	}
}
