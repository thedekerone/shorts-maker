package handlers

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thedekerone/shorts-maker/services"
	"github.com/thedekerone/shorts-maker/services/store"
)

const defaultWebhookBase = "http://localhost:3000"

func generateUniqueName() string {
	timestamp := time.Now().UnixNano()
	uuid := uuid.New().String()
	return fmt.Sprintf("%d_%s", timestamp, uuid)
}

type Job struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	URL       string `json:"url"`
	KlingURL  string `json:"kling_url,omitempty"`
	ImagesURL string `json:"images_url,omitempty"`
	Error     string `json:"error,omitempty"`
}

type CompletedWebhookBody struct {
	ID        string `json:"id"`
	KlingURL  string `json:"kling_url,omitempty"`
	ImagesURL string `json:"images_url,omitempty"`
}

type generationOptions struct {
	GenerateKling  bool
	GenerateImages bool
}

type generationRequest struct {
	Script          string `json:"script"`
	Webhook         string `json:"webhook"`
	CaptionStyle    string `json:"caption_style"`
	CaptionPosition string `json:"caption_position"`
	VoiceID         string `json:"voice_id"`
	Mode            string `json:"mode"`
	MusicID         string `json:"music_id"`
	VisualStyle     string `json:"visual_style"`
}

func cleanURL(u string) string {
	if u == "" {
		return ""
	}
	return strings.ReplaceAll(u, `\u0026`, "&")
}

func (j Job) FormattedURL() string {
	return cleanURL(j.URL)
}

func normalizeMode(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		return "portrait"
	}
	return mode
}

func resolveWebhookURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	return defaultWebhookBase + raw
}

var jobStore *store.Store

func RegisterJobStore(s *store.Store) {
	jobStore = s
}

func setJobVideoURL(jobID, variant, url string) {
	if jobStore == nil || url == "" {
		return
	}
	ctx := context.Background()
	clean := cleanURL(url)
	if err := jobStore.UpdateVideoURL(ctx, jobID, variant, clean); err != nil {
		log.Printf("job %s: failed to update video url: %v", jobID, err)
	}
}

type generationJob struct {
	id        string
	mode      string
	webhook   string
	minio     *services.MinioService
	replicate *services.ReplicateService
	cleanup   []string
}

func newGenerationJob(id, mode, webhook string) *generationJob {
	return &generationJob{id: id, mode: mode, webhook: webhook}
}

func (g *generationJob) initServices() error {
	minioClient, err := connectToMinio(g.id)
	if err != nil {
		return err
	}
	g.minio = minioClient

	rs, err := createReplicateService(g.id)
	if err != nil {
		return err
	}
	g.replicate = rs
	return nil
}

func (g *generationJob) addCleanup(path string) {
	if path == "" {
		return
	}
	g.cleanup = append(g.cleanup, path)
}

func (g *generationJob) cleanupFiles() {
	for _, path := range g.cleanup {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Printf("job %s cleanup %s: %v", g.id, path, err)
		}
	}
}

func (g *generationJob) fail(stage string, err error) error {
	if err == nil {
		return nil
	}
	log.Printf("job %s failed at %s: %v", g.id, stage, err)
	updateJobStatus(g.id, "failed", "", fmt.Sprintf("%s: %v", stage, err))
	return err
}

func updateJobStatus(jobID, status, url, errorMsg string) {
	if jobStore == nil {
		return
	}
	ctx := context.Background()
	if err := jobStore.UpdateStatus(ctx, jobID, status, cleanURL(url), errorMsg); err != nil {
		log.Printf("job %s: failed to update status: %v", jobID, err)
	}
}
