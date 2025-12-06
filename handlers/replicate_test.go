package handlers

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/thedekerone/shorts-maker/models"
)

func TestNormalizeMode(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{"", "portrait"},
		{" Portrait ", "portrait"},
		{"LANDSCAPE", "landscape"},
	}

	for _, tt := range tests {
		if got := normalizeMode(tt.in); got != tt.out {
			t.Fatalf("normalizeMode(%q) = %q, want %q", tt.in, got, tt.out)
		}
	}
}

func TestResolveWebhookURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{"empty", "", ""},
		{"relative", "/hook", defaultWebhookBase + "/hook"},
		{"absolute", "https://example.com/h", "https://example.com/h"},
	}

	for _, tt := range tests {
		if got := resolveWebhookURL(tt.in); got != tt.out {
			t.Fatalf("resolveWebhookURL(%s) = %q, want %q", tt.name, got, tt.out)
		}
	}
}

func TestFilterEmpty(t *testing.T) {
	input := []string{"a", " ", "b"}
	want := []string{"a", "b"}
	got := filterEmpty(input)
	if len(got) != len(want) {
		t.Fatalf("filterEmpty len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("filterEmpty[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBuildKlingSegments(t *testing.T) {
	stills := []models.ImageWithTimestamp{
		{URL: "one.png", Timestamp: 3, Prompt: "A"},
		{URL: "two.png", Timestamp: 3, Prompt: "B"},
		{URL: "three.png", Timestamp: 4, Prompt: "C"},
		{URL: "four.png", Timestamp: 2, Prompt: "D"},
	}

	segments := buildKlingSegments(stills)
	if len(segments) == 0 {
		t.Fatalf("expected segments")
	}
	for _, seg := range segments {
		if seg.Duration != 5 && seg.Duration != 10 {
			t.Fatalf("segment duration %d invalid", seg.Duration)
		}
		if seg.StartImage.URL == "" {
			t.Fatalf("segment missing start image")
		}
		if seg.Prompt == "" {
			t.Fatalf("segment missing prompt")
		}
	}
}

func TestSetJobVideoURL(t *testing.T) {
	job := &Job{ID: "job1"}
	jobsMutex.Lock()
	jobs[job.ID] = job
	jobsMutex.Unlock()

	setJobVideoURL(job.ID, "kling_video", "https://foo")
	if job.KlingURL == "" || job.URL != job.KlingURL {
		t.Fatalf("kling url not set properly: %+v", job)
	}

	setJobVideoURL(job.ID, "image_video", "https://bar")
	if job.ImagesURL != "https://bar" {
		t.Fatalf("image url not set")
	}
}

func TestParseGenerationRequest(t *testing.T) {
	body := `{"script":"hello\nworld","webhook":"/hook","voice_id":"v","mode":"Landscape"}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	got, err := parseGenerationRequest(req)
	if err != nil {
		t.Fatalf("parseGenerationRequest error: %v", err)
	}
	if got.Script != "hello world" {
		t.Fatalf("script cleaned mismatch: %q", got.Script)
	}
	if got.Webhook != defaultWebhookBase+"/hook" {
		t.Fatalf("webhook mismatch: %q", got.Webhook)
	}
	if got.Mode != "landscape" {
		t.Fatalf("mode mismatch: %q", got.Mode)
	}
}

func TestParseGenerationRequestRequiresScript(t *testing.T) {
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"script":""}`))
	if _, err := parseGenerationRequest(req); err == nil {
		t.Fatal("expected error for empty script")
	}
}
