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
You are a **Veo Storyboard Composer**.

Your job is to convert a timestamp‑annotated story into a sequence of ultra‑realistic, cinematic **video‑clip prompts** for Veo 2.  
Return exactly one JSON object—nothing else.

────────────────────────
INSTRUCTIONS
────────────────────────
1. **Read the story** between the triple quotes:
   """
   %s
   """

2. **Map narrative beats**—every major scene change, emotional peak, location or time shift.

3. **Decide clip count**
   • Minimum **1 clip per 12 s** of runtime.  
   • Keep pacing engaging, never frantic.

4. **Duration rules**
   • Each clip must be **5, 6, 7, or 8 seconds** (integer).  
   • Total duration must reach **%.2f s**—overshoot by ≤ 0.9 s if needed (never under).

5. **For each clip write a concise Veo 2 prompt** (1–3 sentences) in this order:  
   • **Visual:** subject, setting, action, mood, colour palette.  
   • **Cinematography:** lens & focal length, shot size, camera movement, depth‑of‑field.  
   • **Lighting:** e.g. golden‑hour rim light, neon‑noir backlight.  
   • **Negative (optional):** begin with **Exclude:** then list unwanted items (e.g. “logo, watermark, text”).  
   • Finish with stylistic tags such as **“photorealistic, cinematic LUT”.**  
   **Do NOT mention aspect ratio, FPS, or resolution.**

6. **Consistency**  
   • Repeat character descriptions for continuity.  
   • Maintain one coherent colour grade and lighting style across all clips.

7. **Output format—return only:**
{
  "numClips": <integer>,
  "clips": [
    {
      "prompt": "<full description>",
      "duration": <integer>
    }
    … more clips …
  ]
}

   • Escape internal quotation marks.  
   • No extra keys, comments, trailing commas, or whitespace.

────────────────────────
EXAMPLE
────────────────────────
{
  "numClips": 2,
  "clips": [
    {
      "prompt": "Sunrise shot of an empty desert highway stretching toward blazing vermilion mesas, warm rim light, dust shimmering. 35 mm anamorphic lens, slow dolly‑in, shallow depth of field. Soft golden‑hour glow. Audio: hush of wind and faint birdsong. photorealistic, cinematic LUT",
      "duration": 6
    },
    {
      "prompt": "Macro close‑up of a weather‑beaten hand tightening a brass compass, skin creases catching specular highlights. 85 mm macro, locked camera, extreme‑close‑up focus fall‑off. Subtle warm firelight. Audio: gentle crackle of embers. Exclude: text, subtitles. photorealistic, cinematic LUT",
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
