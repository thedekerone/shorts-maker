package images

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thedekerone/shorts-maker/elevenlabs"
	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/services"
)

type imagePromptResp struct {
	NumImages int `json:"numImages"`
	Images    []struct {
		Prompt         string  `json:"prompt"`
		NegativePrompt string  `json:"negative_prompt,omitempty"`
		Duration       float64 `json:"duration,omitempty"` // ignored; we lock to ShotPlan
		Seed           *int    `json:"seed,omitempty"`
	} `json:"images"`
}

func GetImagesWithTimestamps(shotPlan *elevenlabs.ShotPlan, mode string) ([]models.ImageWithTimestamp, error) {
	if shotPlan == nil || len(shotPlan.Shots) == 0 {
		return nil, fmt.Errorf("empty shot plan")
	}

	// one client does both: completion + image gen
	rs, err := services.NewReplicateService()
	if err != nil {
		return nil, fmt.Errorf("create replicate service: %w", err)
	}

	// build a clean JSON INPUT for the model (timeline is fixed to the plan)
	type inShot struct {
		Index int     `json:"index"`
		Start float64 `json:"start"`
		End   float64 `json:"end"`
		Text  string  `json:"text"`
	}
	in := struct {
		TotalDuration float64  `json:"total_duration"`
		Shots         []inShot `json:"shots"`
	}{
		TotalDuration: shotPlan.TotalEnd,
		Shots:         make([]inShot, len(shotPlan.Shots)),
	}
	for i, s := range shotPlan.Shots {
		in.Shots[i] = inShot{Index: i, Start: s.Start, End: s.End, Text: strings.TrimSpace(s.Text)}
	}
	inputJSON, _ := json.Marshal(in)

	system := `You are an Image-Prompt Composer.

Your job: turn a timestamped story broken into shots into a sequence of ultra-realistic, cinematic IMAGE PROMPTS.
Return a single JSON object and nothing else.

Each image is generated independently (no shared state). To keep visuals cohesive, you must repeat the same Style Anchor and character details in every prompt.

HARD RULES
- No camera tech: don’t mention cameras, lenses, focal lengths, apertures, ISO, shutter speed, or depth-of-field.
- Keep a consistent look across all images: art direction, palette, lighting ethos, texture/grain, aspect ratio.
- Use clear labeled sections inside each prompt; simple, declarative phrasing.
- No pronouns for recurring subjects; restate names and fixed traits every time.
- Output ONLY strict JSON (no markdown, no comments, no extra text).

INPUT FORMAT (from the user)
You will receive a list of shots with text, start, and end timestamps, and may also receive a total_duration. Example:
{ shot: "<text>", start: <float>, end: <float> }
{ shot: "<text>", start: <float>, end: <float> }
...

INSTRUCTIONS
1) Read all shots and infer the story’s mood & genre (e.g., hopeful, melancholic, tense; drama, thriller, adventure).
2) Choose a matching Style Family (e.g., “gritty neo-noir”, “warm nostalgic drama”, “cold techno-thriller”, “sun-bleached road movie”) that supports that mood.
3) Create a single STYLE ANCHOR to use verbatim in every prompt:
   • Art direction: “photorealistic, cinematic, natural materials, subtle film grain”.
   • Style family (from step 2) and a short reason it fits the mood.
   • Color palette: fixed hues/accents (e.g., “teal and amber highlights, muted neutrals”).
   • Lighting ethos: general terms only (e.g., “soft directional sunlight with gentle rim light” or “overcast diffuse light”).
   • Aspect ratio: “16:9” (use a different ratio only if the story clearly demands it).
   • Texture: “hyper-real surface detail, clean edges”.
   • Negatives: “no text, no watermark, no logo, no extra fingers, normal human anatomy, no motion blur, no distortion”.
4) Build CHARACTER SHEETS for each recurring subject (name + immutable traits: age, ethnicity, facial structure, hair, eyes, build, wardrobe items/colors, signature props).
   Use the same wording for these traits every time that character appears.
5) Shot planning & pacing
   - Produce exactly one image per input shot, in the same order.
   - If a shot provides start and end, set that image’s duration = end − start (round to 2 decimals).
   - If total_duration is provided and shot timestamps are missing, pace ~6–8s per image and ensure the sum of durations equals total_duration (adjust the final item to correct rounding drift).
6) For each image, write a stand-alone prompt with these labeled sections (labels included in the text):
   • STYLE ANCHOR: <paste the exact Style Anchor text verbatim>
   • CHARACTERS: <repeat full name + fixed traits + wardrobe for all visible recurring characters>
   • SCENE: <setting, time of day, weather, key props, physical actions; objective description only>
   • LIGHTING: <use the lighting ethos; describe direction/quality/intensity without camera jargon>
   • COMPOSITION: <framing only: “wide establishing view”, “medium two-shot”, “tight portrait”, “over-shoulder”, “low angle”, “symmetrical composition”>
   • MOOD CUES: <one plain sentence stating the emotional tone>
   • QUALITY TAGS: “8K, photorealistic, hyper-real textures, cinematic color grade”
   • NEGATIVES: <repeat negatives from the Style Anchor>
   Keep each prompt ≤ 70 words (concise and parsable).

OUTPUT FORMAT (strict JSON only)
{
  "numImages": <integer>,
  "images": [
    {
      "prompt": "<STYLE ANCHOR: …> <CHARACTERS: …> <SCENE: …>  <NEGATIVES: …>",
      "duration": <float>
    }
    // one object per shot, in order
  ]
}

VALIDATION
- numImages must equal the number of input shots.
- If shot timestamps are present, each duration must equal end − start (rounded to two decimals).
- If total_duration is provided, the sum of durations must equal total_duration (±0.01). Correct any rounding drift on the final item.

(Strict mode option: If shot timestamps are present, you must not change durations; use exactly end − start.)
`

	userPrompt := "INPUT:\n" + string(inputJSON)

	raw, err := rs.GetCompletition(userPrompt, system)
	if err != nil {
		return nil, fmt.Errorf("completion error: %w", err)
	}

	raw = stripJSON(raw) // peel code fences / chatter if any
	var parsed imagePromptResp
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("LLM returned invalid JSON: %w\nraw: %s", err, raw)
	}
	if len(parsed.Images) != len(shotPlan.Shots) {
		return nil, fmt.Errorf("LLM prompt count mismatch: got %d for %d shots", len(parsed.Images), len(shotPlan.Shots))
	}

	out := make([]models.ImageWithTimestamp, 0, len(parsed.Images))
	for i, p := range parsed.Images {
		prompt := strings.TrimSpace(p.Prompt)
		if prompt == "" {
			// degrade gracefully to the shot text
			prompt = strings.TrimSpace(shotPlan.Shots[i].Text)
		}

		urls, err := rs.GetImages(prompt, 1, mode)
		if err != nil {
			return nil, fmt.Errorf("image %d: %w", i, err)
		}
		if len(urls) == 0 || urls[0] == "" {
			return nil, fmt.Errorf("image %d: empty result", i)
		}

		// Use your plan’s timing (duration = End-Start). If your field is truly a duration,
		// consider renaming models.ImageWithTimestamp.Timestamp -> Duration.
		dur := shotPlan.Shots[i].End - shotPlan.Shots[i].Start
		out = append(out, models.ImageWithTimestamp{
			URL:       urls[0],
			Timestamp: dur,
		})
	}

	return out, nil
}

// stripJSON tries to return the first {...} block (helps when models add code fences or chatter).
func stripJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.Index(s, "{"); i >= 0 {
			if j := strings.LastIndex(s, "}"); j > i {
				return s[i : j+1]
			}
		}
	}
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return s
}
