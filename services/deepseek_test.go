package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetCompletitionForImages(t *testing.T) {
	t.Helper()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header %q", got)
		}

		var req DeepSeekRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model == "" || len(req.Messages) != 2 {
			t.Fatalf("unexpected request payload: %+v", req)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"numImages\":1,\"stylePrompt\":\"cinematic\",\"images\":[{\"prompt\":\"rain\",\"duration\":6}]}"}}]}`))
	}))
	t.Cleanup(ts.Close)

	svc := &DeepSeekService{
		APIKey:     "test-key",
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
	}

	res, err := svc.GetCompletitionForImages("prompt", "system")
	if err != nil {
		t.Fatalf("GetCompletitionForImages error: %v", err)
	}

	if res.StylePrompt != "cinematic" {
		t.Fatalf("unexpected style prompt %q", res.StylePrompt)
	}
	if len(res.ImagesPrompt) != 1 || res.ImagesPrompt[0].Prompt != "rain" {
		t.Fatalf("unexpected image prompts: %+v", res.ImagesPrompt)
	}
}

func TestGetCompletitionForClips_ErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	t.Cleanup(ts.Close)

	svc := &DeepSeekService{
		APIKey:     "key",
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
	}

	if _, err := svc.GetCompletitionForClips("prompt", "system"); err == nil {
		t.Fatal("expected error for non-200 response")
	}
}
