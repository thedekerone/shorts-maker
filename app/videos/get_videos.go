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
	println(vs.Location)

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

	imageGenerationPrompts := fmt.Sprintf(
		`
		### SYSTEM ###
You are an **Image‑Prompt Composer**.

Your job is to turn a timestamped story into a sequence of ultra‑realistic, cinematic image prompts—returned as a single JSON object and nothing else.

INSTRUCTIONS
1. Read the story supplied between the triple quotes: 
   """
   %s
   """
2. **Identify key moments** (scene changes, emotional peaks, environment shifts).
3. Decide the number of images:  
   • ≥ 1 image every 12 s.  
   • Keep pacing engaging, not frantic.  
4. Allocate each image’s on‑screen **duration** so that the sum equals %.2f (±0.01 s).
5. For every image craft a **stand‑alone prompt** that fully describes:  
   • Setting, subjects, action, mood.  
   • Lighting style (e.g., golden‑hour rim light).  
   • Camera details (lens, depth‑of‑field, framing, shot type).  
   • Stylistic tags: “8 K, photorealistic, cinematic color grade”.  
   (Assume the generator has no other context.)
6. Maintain a *single, coherent visual style* across all images—hyper‑real textures, lifelike lighting.
7. Output **only** the JSON below (no code fences, no comments).

OUTPUT FORMAT
{
  "numImages": <integer>,
  "images": [
    {
      "prompt": "<full scene description>",
      "duration": <float>   // seconds
    }
    // … additional images …
  ]
}

EXAMPLE
{
  "numImages": 3,
  "images": [
    {
      "prompt": "Wide‑angle sunrise shot of an isolated desert road stretching toward crimson mountains, warm golden‑hour light casting long shadows, crisp 50 mm lens, shallow depth of field, hyper‑realistic 8 K, cinematic color grade",
      "duration": 11.5
    },
    {
      "prompt": "Macro close‑up of a weathered hand gripping a rusty compass, soft ambient backlight revealing skin texture, f/2.8, filmic grain, photorealistic 8 K",
      "duration": 12.0
    },
    {
      "prompt": "Lone traveler silhouetted beneath a vast starlit sky on a windswept plateau, cool moonlight, slow dolly‑out 35 mm, HDR, ultra‑real 8 K",
      "duration": 13.0
    }
  ]
}

`, fmt.Sprintf("\n %v", segmentStrings), totalDuration)

	// Same JSON-builder prompt you used for images
	prompt := fmt.Sprintf(imageGenerationPrompts, segmentStrings, totalDuration)

	imgPlan, err := deepseek.GetCompletitionForImages(prompt, "")
	if err != nil {
		return nil, fmt.Errorf("deepseek completion: %w", err)
	}

	// ── 3.  Loop through each “image” entry, but request a clip instead───
	var clips []models.VideoWithTimestamp
	for i, entry := range imgPlan.ImagesPrompt {
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
			Path:      paths[0],       // local tmp .mp4
			Timestamp: entry.Duration, // original storyboard time
		})
	}
	return clips, nil
}
