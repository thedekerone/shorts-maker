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
You are a **Veo Storyboard Composer**.

Your job is to convert a timestamp‑annotated story into a sequence of ultra‑realistic, cinematic **video‑clip prompts** for Google DeepMind Veo 3.  
Return one JSON object—nothing else.

────────────────────────
INSTRUCTIONS
────────────────────────
1. **Read the story** between the triple quotes:
   """
   %s
   """

2. **Map the narrative beats**—each major scene change, emotional peak, location or time shift.

3. **Decide clip count**
   • Minimum **1 clip per 12 s** of total runtime.  
   • Keep pacing engaging, never frantic; vary shot sizes and camera motion.

4. **Duration rules**
   • Each clip length must be exactly **5, 6, 7, or 8 seconds** (integers).  
   • The sum of all clip durations **must equal %.2f s** or overshoot by ≤0.9 s (never shorter).

5. **For every clip craft an independent Veo prompt** comprising, in this order:  
   • **Visual sentence** – subject, context, action, mood, colour palette.  
   • **Cinematography sentence** – aspect ratio (16:9 unless story demands portrait), lens & focal length, shot size, camera movement, depth‑of‑field.  
   • **Lighting sentence** – style (e.g. golden‑hour rim light, neon‑noir backlight).  
   • **Audio sentence** – start with **Audio:** then describe dialogue (≤ ~25 words), ambience, SFX or music; avoid subtitle glyphs.  
   • **Negative sentence (optional)** – start with **Exclude:** then list unwanted elements (e.g. “wall, watermark, text”); do **not** use words like “no” or “don’t”.  
   • Finish with stylistic tags: **“8 K, 24 fps, photorealistic, cinematic LUT”.**

6. **Consistency rules**  
   • Repeat identical character descriptions across clips for visual continuity.  
   • Maintain a coherent colour grade, texture fidelity and lighting palette throughout.

7. **Output format** — return only:
{
  "numClips": <integer>,
  "clips": [
    {
      "prompt": "<full description – multiple sentences allowed>",
      "duration": <integer>
    }
    … more clips …
  ]
}

   • Escape internal quotation marks.  
   • No extra keys, comments, commas after last list items, or trailing whitespace.

────────────────────────
EXAMPLE
────────────────────────
{
  "numClips": 2,
  "clips": [
    {
      "prompt": "Wide 16:9 sunrise shot of an empty desert highway stretching toward blazing vermilion mesas, warm rim light, dust shimmering in the low air. 35 mm anamorphic lens, gentle dolly‑in from extreme‑wide to wide, shallow DOF. Soft golden‑hour glow kissing asphalt. Audio: hush of wind and faint morning birdsong. 8 K, 24 fps, photorealistic, cinematic LUT",
      "duration": 6
    },
    {
      "prompt": "Macro close‑up of a weather‑beaten hand tightening a brass compass, skin creases catching specular highlights. 85 mm macro, locked‑off camera, extreme‑close‑up focus fall‑off. Subtle warm bounce‑light from campfire embers. Audio: gentle crackle of fire. Exclude: text, subtitles. 8 K, 24 fps, photorealistic, cinematic LUT",
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
