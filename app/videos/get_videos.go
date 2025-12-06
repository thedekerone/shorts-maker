// videos/get_videos.go
package videos

import (
	"fmt"
	"os"

	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/prompts"
	"github.com/thedekerone/shorts-maker/services"
)

// mode is still "portrait" | "landscape" in case you want 9:16 vs 16:9
func GetVideosWithTimestamps(
	transcript *models.TranscriptionOutput,
	mode string,
) ([]models.VideoWithTimestamp, error) {

	// ── 1.  Open services ────────────────────────────────────────────────
	promptSvc, err := services.NewPromptService()
	if err != nil {
		return nil, fmt.Errorf("init prompt service: %w", err)
	}

	// Initialise VeoService.  Keep IDs in env vars or config.
	vs, err := services.NewVeoService(
		os.Getenv("PROJECT_ID"), // e.g. "cortos-ai-466916"
		"us-central1",
		"veo-2.0-generate-001",
	)
	println(vs.Location)

	if err != nil {
		return nil, fmt.Errorf("init veo: %w", err)
	}

	// ── 2.  Build the storyboard prompt ─────────────────────────────────
	totalDuration := transcript.Segments[len(transcript.Segments)-1].End
	var segmentStrings string
	for _, s := range transcript.Segments {
		segmentStrings += fmt.Sprintf("{ segment: %s, start: %.3f, end: %.3f }\n",
			s.Text, s.Start, s.End)
	}

	systemPrompt := prompts.ClipSystemPrompt()
	userPrompt := fmt.Sprintf("STORY_SEGMENTS:\n%s\n\nTOTAL_DURATION: %.2f", segmentStrings, totalDuration)

	clipPlan, err := promptSvc.GenerateClipPlan(userPrompt, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("prompt completion: %w", err)
	}

	// ── 3.  Loop through each clip entry and request a Veo generation ─────
	var clips []models.VideoWithTimestamp
	for i, entry := range clipPlan.Clips {
		println("trying to generate clip")
		// constrain each clip to max 8 s (Veo-2 limit)
		dur := entry.Duration
		if dur > 8 {
			dur = 8
		}

		// portrait vs landscape decides aspect ratio only – pick res
		res := "720p"
		if mode == "landscape" {
			res = "720p" // same – Veo-2 ignores resolution anyway
		}

		paths, err := vs.GetVideos(entry.Prompt, int(dur), res, 1)
		println(paths)
		if err != nil {
			println(err)
			return nil, fmt.Errorf("clip %d: %w", i+1, err)
		}
		if len(paths) == 0 {
			continue
		}

		clips = append(clips, models.VideoWithTimestamp{
			Path:      paths[0],                // local tmp .mp4
			Timestamp: float64(entry.Duration), // original storyboard time
		})
	}
	return clips, nil
}
