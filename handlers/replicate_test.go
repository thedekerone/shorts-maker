package handlers

import (
	"bytes"
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/services/store"
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
	st := setupTestStore(t)
	ctx := context.Background()
	jobID := "job1"
	if err := st.CreateJob(ctx, store.JobRecord{ID: jobID, Script: "script", Mode: "portrait", Status: "queued"}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	setJobVideoURL(jobID, "kling_video", "https://foo")
	record, err := st.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if record.KlingURL == "" || cleanURL(record.KlingURL) != "https://foo" || record.CurrentURL != "https://foo" {
		t.Fatalf("kling url not stored: %+v", record)
	}

	setJobVideoURL(jobID, "image_video", "https://bar")
	record, err = st.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if cleanURL(record.ImagesURL) != "https://bar" {
		t.Fatalf("image url not set: %+v", record)
	}
}

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "jobs.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	RegisterJobStore(st)
	t.Cleanup(func() {
		st.Close()
		RegisterJobStore(nil)
	})
	return st
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
