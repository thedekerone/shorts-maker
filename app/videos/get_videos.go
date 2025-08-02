// videos/get_videos.go
package videos

import (
	"fmt"
	"os"

	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/services"
)

// mode is still "portrait" | "landscape" in case you want 9:16 vs 16:9
func GetVideosWithTimestamps(
	transcript *models.TranscriptionOutput,
	mode string,
) ([]models.VideoWithTimestamp, error) {

	// ── 1.  Open services ────────────────────────────────────────────────
	deepseek, err := services.NewDeepSeekService()
	if err != nil {
		return nil, fmt.Errorf("init deepseek: %w", err)
	}

	// Initialise VeoService.  Keep IDs in env vars or config.
	vs, err := services.NewVeoService(
		os.Getenv("PROJECT_ID"), // e.g. "cortos-ai-466916"
		"us-central1",
		"veo-2.0-generate-001",
	)
	if err != nil {
		return nil, fmt.Errorf("init veo: %w", err)
	}

	// ── 2.  Build the prompt for DeepSeek (unchanged) ────────────────────
	totalDuration := transcript.Segments[len(transcript.Segments)-1].End
	var segmentStrings string
	for _, s := range transcript.Segments {
		segmentStrings += fmt.Sprintf("{ segment: %s, start: %.3f, end: %.3f }\n",
			s.Text, s.Start, s.End)
	}

	imageGenerationPromptsTemplate := fmt.Sprintf(`
		### SYSTEM ###
You are a **Video-Prompt Composer**.

Your job is to turn a timestamped story into a sequence of ultra-realistic, cinematic **video-clip prompts**—returned as a single JSON object and nothing else.

INSTRUCTIONS  
1. Read the story supplied between the triple quotes:  
   """  
   %s  
   """  

2. **Identify key moments** (scene changes, emotional peaks, environment shifts).

3. Decide the number of clips:  
   • Each clip **must** last **5 – 8 s**.  
   • Pace the story so it feels engaging, not frantic.

4. Allocate every clip’s **duration** (float) so that the **sum equals %.2f s (± 0.01 s)**.  
   • Stay within the 5 – 8 s window for every individual clip.

5. For every clip craft a **stand-alone prompt** that fully describes:  
   • Setting, subjects, action, and mood.  
   • Lighting style (e.g., golden-hour rim light).  
   • Camera details (lens, depth-of-field, framing, shot type, motion—pan, dolly, aerial, etc.).  
   • Stylistic tags: “8 K, 30 fps, photorealistic, cinematic color grade”.  
   (Assume the generator has no other context.)

6. Maintain a *single, coherent visual style* across all clips—hyper-real textures, lifelike lighting.

7. Output **only** the JSON below (no code fences, no comments).

OUTPUT FORMAT
{
  "numClips": <integer>,
  "clips": [
    {
      "prompt": "<full scene description>",
      "duration": <float>   // seconds (5-8)
    }
    // … additional clips …
  ]
}

EXAMPLE
{
  "numClips": 2,
  "clips": [
    {
      "prompt": "Slow dolly-in at sunrise over an isolated desert road stretching toward crimson mountains, warm golden-hour light casting long shadows, crisp 50 mm lens, shallow depth of field, photorealistic 8 K, 30 fps, cinematic color grade",
      "duration": 6.5
    },
    {
      "prompt": "Aerial tracking shot of a lone traveler silhouetted against a vast starlit sky on a windswept plateau, cool moonlight, subtle gimbal motion, 35 mm equivalent, HDR, photorealistic 8 K, 30 fps",
      "duration": 7.2
    }
  ]
}

		`, fmt.Sprintf("\n %v", segmentStrings), totalDuration)

	// Same JSON-builder prompt you used for images
	prompt := fmt.Sprintf(imageGenerationPromptsTemplate, segmentStrings, totalDuration)

	imgPlan, err := deepseek.GetCompletitionForImages(prompt, "")
	if err != nil {
		return nil, fmt.Errorf("deepseek completion: %w", err)
	}

	// ── 3.  Loop through each “image” entry, but request a clip instead───
	var clips []models.VideoWithTimestamp
	for i, entry := range imgPlan.ImagesPrompt {
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
		if err != nil {
			return nil, fmt.Errorf("clip %d: %w", i+1, err)
		}
		if len(paths) == 0 {
			continue
		}

		clips = append(clips, models.VideoWithTimestamp{
			Path:      paths[0],       // local tmp .mp4
			Timestamp: entry.Duration, // original storyboard time
		})
	}
	return clips, nil
}
