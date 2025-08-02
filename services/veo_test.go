//go:build integration
// +build integration

package services_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thedekerone/shorts-maker/services"
)

func hasADC() bool {
	home := os.Getenv("HOME")
	if home == "" {
		return false
	}
	_, err := os.Stat(
		filepath.Join(home, ".config", "gcloud", "application_default_credentials.json"),
	)
	return err == nil
}

func _TestVeoService_StartGeneration(t *testing.T) {
	if os.Getenv("CLOUDSDK_AUTH_ACCESS_TOKEN") == "" && !hasADC() {
		t.Skip("no auth token or ADC; skipping integration test")
	}
	project := os.Getenv("PROJECT_ID")
	if project == "" {
		t.Skip("PROJECT_ID not set")
	}

	vs, err := services.NewVeoService(project, "us-central1", "veo-2.0-generate-001")
	if err != nil {
		t.Fatalf("NewVeoService: %v", err)
	}

	op, err := vs.StartGeneration(
		"A sunrise time-lapse over a mountain lake, hyper-realistic",
		8, "", 1,
	)
	if err != nil {
		t.Fatalf("StartGeneration error: %v", err)
	}
	if op == "" {
		t.Fatal("expected non-empty operation name")
	}
}

func TestVeoService_PollGeneration(t *testing.T) {
	if os.Getenv("CLOUDSDK_AUTH_ACCESS_TOKEN") == "" && !hasADC() {
		t.Skip("no auth token or ADC; skipping integration test")
	}
	project := os.Getenv("PROJECT_ID")
	if project == "" {
		t.Skip("PROJECT_ID not set")
	}

	vs, _ := services.NewVeoService(project, "us-central1", "veo-2.0-generate-001")

	// Kick off a job (could take minutes).
	op, err := vs.StartGeneration(
		"A macro shot of raindrops rolling off a leaf in slow motion",
		8, "", 1,
	)
	if err != nil {
		t.Fatalf("StartGeneration: %v", err)
	}

	// Now poll until the video is ready.
	paths, err := vs.PollGeneration(op)
	if err != nil {
		t.Fatalf("PollGeneration: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one video path")
	}
	// Clean up.
	for _, p := range paths {
		_ = os.Remove(p)
	}
}
