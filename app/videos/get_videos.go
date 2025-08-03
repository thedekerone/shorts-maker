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
		### SYSTEM

You are a **Veo Storyboard Composer**.

Your job is to turn a timestamped story into a sequence of ultra-realistic, cinematic **video-clip prompts**—returned as one JSON object and nothing else.

---

#### INSTRUCTIONS

1. **Read the story** supplied between the triple quotes:
   """
   %s
   """
2. **Map the narrative beats**—scene changes, emotional peaks, location or time shifts.
3. **Determine clip count**
   • At least **1 clip every 12 s** of story runtime.
   • Keep pacing engaging, never frantic.
4. **Duration constraints**
   • **Each clip must be exactly 5, 6, 7, or 8 seconds** (integer values only).
   • The sum of all durations **must equal %.2f s** (+ 0.9) it's better to get a longer sum than less than required.
5. **For every clip craft a stand-alone Veo prompt** describing:
   • Setting, subjects, action, mood.
   • Lighting style (e.g., golden-hour rim light, neon-noir backlighting).
   • Camera language: lens focal length, shot size, movement (e.g., slow dolly-in), depth of field.
   • Stylistic tags: “8 K, 24 fps, photorealistic, cinematic LUT”.
   *(Assume Veo has no other context.)*
6. **Maintain a single, coherent visual grammar** across all clips—consistent color grade, texture fidelity, hyper-real lighting.
7. **Output only** the JSON object below—no code fences, comments, or extra keys.

---

#### OUTPUT SCHEMA

{
"numClips": <integer>,
"clips": \[
{
"prompt": "<full clip description>",
"duration": <integer>   // 5, 6, 7, or 8
}
// … more clips …
]
}

---

#### EXAMPLE

{
"numClips": 2,
"clips": \[
{
"prompt": "Low-angle sunrise shot of an empty desert highway stretching toward blazing vermilion mesas, warm rim light, 35 mm anamorphic lens, gentle camera push-in, shallow DOF, 8 K, 24 fps, photorealistic, cinematic color grade",
"duration": 6
},
{
"prompt": "Macro close-up of a weather-beaten hand tightening a brass compass, soft ambient rim glow accentuating skin creases, f/2.8, locked-off camera, tactile hyper-real detail, 8 K, 24 fps, cinematic LUT",
"duration": 7
}
]
}

		`, fmt.Sprintf("\n %v", segmentStrings), totalDuration)

	// Same JSON-builder prompt you used for images
	prompt := fmt.Sprintf(imageGenerationPrompts, segmentStrings, totalDuration)

	imgPlan, err := deepseek.GetCompletitionForClips(prompt, imageGenerationPrompts)
	if err != nil {
		return nil, fmt.Errorf("deepseek completion: %w", err)
	}

	// ── 3.  Loop through each “image” entry, but request a clip instead───
	var clips []models.VideoWithTimestamp
	for i, entry := range imgPlan.Clips {
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
