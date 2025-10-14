package images

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/thedekerone/shorts-maker/elevenlabs"
	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/services"
)

// Same local response type you already had:
type imagePromptResp struct {
	NumImages int `json:"numImages"`
	Images    []struct {
		Prompt         string  `json:"prompt"`
		NegativePrompt string  `json:"negative_prompt,omitempty"`
		Duration       float64 `json:"duration,omitempty"`
		Seed           *int    `json:"seed,omitempty"`
	} `json:"images"`
}

type inShot struct {
	Index int     `json:"index"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

func GetImagesWithTimestamps(shotPlan *elevenlabs.ShotPlan, mode string) ([]models.ImageWithTimestamp, error) {
	if shotPlan == nil || len(shotPlan.Shots) == 0 {
		return nil, fmt.Errorf("empty shot plan")
	}

	// Keep your image-gen service (replicate) for the actual images
	rs, err := services.NewReplicateService()
	if err != nil {
		return nil, fmt.Errorf("create replicate service: %w", err)
	}

	// Use DeepSeek for completions
	ds, err := services.NewDeepSeekService()
	if err != nil {
		return nil, fmt.Errorf("create deepseek service: %w", err)
	}

	// Build clean JSON of the entire plan (used in context + origin)
	type modelInput struct {
		TotalDuration float64  `json:"total_duration"`
		Shots         []inShot `json:"shots"`
	}
	in := modelInput{
		TotalDuration: shotPlan.TotalEnd,
		Shots:         make([]inShot, len(shotPlan.Shots)),
	}
	for i, s := range shotPlan.Shots {
		in.Shots[i] = inShot{Index: i, Start: s.Start, End: s.End, Text: strings.TrimSpace(s.Text)}
	}
	inputJSON, _ := json.Marshal(in)
	inputHash := shortHash(inputJSON)

	// ---- 1) Build CONTEXT via DeepSeek (Style Anchor + Characters) ----
	ctxSystem := `You are an Image-Prompt Context Builder.

Task: From the full shot list (JSON), extract the global STYLE ANCHOR and CHARACTER SHEETS ONLY.
Return STRICT JSON only:
{
  "style_anchor": "<text>",
  "characters": ["<fixed sheet #1>", "<fixed sheet #2>", ...]
}

Rules:
- STYLE ANCHOR: art direction, style family (+short reason), color palette, lighting ethos, aspect ratio (16:9 unless story demands), texture, negatives.
- CHARACTER SHEETS: immutable wording (name, age, ethnicity, facial structure, hair, eyes, build, wardrobe items/colors, signature props).
- No per-shot prompts here.`

	ctxUser := map[string]any{
		"origin": map[string]any{
			"package":    "images",
			"function":   "GetImagesWithTimestamps",
			"stage":      "context_build",
			"input_hash": inputHash,
		},
		"input": json.RawMessage(inputJSON),
	}
	ctxUserJSON, _ := json.Marshal(ctxUser)

	rawCtx, err := ds.GetCompletionRaw("INPUT:\n"+string(ctxUserJSON), ctxSystem, 6000)
	if err != nil {
		return nil, fmt.Errorf("context completion error: %w", err)
	}
	rawCtx = stripJSON(rawCtx)

	var ctx struct {
		StyleAnchor string   `json:"style_anchor"`
		Characters  []string `json:"characters"`
	}
	if err := json.Unmarshal([]byte(rawCtx), &ctx); err != nil {
		return nil, fmt.Errorf("context JSON invalid: %w\nraw: %s", err, rawCtx)
	}
	if strings.TrimSpace(ctx.StyleAnchor) == "" {
		return nil, fmt.Errorf("context missing style_anchor")
	}

	// ---- 2) Per-shot: call DeepSeek.GetCompletitionForImages with numImages=1 ----
	out := make([]models.ImageWithTimestamp, 0, len(in.Shots))

	perShotSystem := `You are an Image-Prompt Composer.

Return STRICT JSON ONLY matching this schema:
{
  "numImages": 1,
  "images": [
    { "prompt": "<STYLE ANCHOR: …> <CHARACTERS: …> <SCENE: …> <NEGATIVES: …>", "duration": <float> }
  ]
}

Rules:
- Use provided context (style_anchor + characters) VERBATIM (no changes).
- No camera tech (no lens/focal/aperture/ISO/shutter/DoF).
- ≤ 70 words per prompt.
- NEGATIVES must repeat from style_anchor.
- Duration MUST equal "strict_duration" (float).`

	for i, shot := range in.Shots {
		shotPayload := map[string]any{
			"origin": map[string]any{
				"package":     "images",
				"function":    "GetImagesWithTimestamps",
				"mode":        mode,
				"shot_index":  shot.Index,
				"total_shots": len(in.Shots),
				"input_hash":  inputHash,
			},
			"context": ctx,
			"shot": map[string]any{
				"index": shot.Index,
				"start": shot.Start,
				"end":   shot.End,
				"text":  shot.Text,
			},
			"strict_duration": round2(shot.End - shot.Start),
		}
		ujson, _ := json.Marshal(shotPayload)

		// DeepSeek's image completion (we constrain it to numImages=1 via system+user)
		// NOTE: This returns an ImagePromptGenerator-shaped JSON, which we parse locally.
		resp, err := ds.GetCompletitionForImages("SHOT_INPUT:\n"+string(ujson), perShotSystem)
		if err != nil {
			return nil, fmt.Errorf("completion (shot %d) error: %w", i, err)
		}
		// Safety: ensure shape & defaults
		if resp == nil || resp.NumImages != 0 { // ignore Clip schema; just sanity
			// continue; nothing to do
		}

		// Convert the DeepSeek response (ImagePromptGenerator shape) into our local imagePromptResp
		// The method already tried to unmarshal into services.ImagePromptGenerator; to stay decoupled,
		// re-marshal and unmarshal into our local struct.
		b, _ := json.Marshal(resp)
		var parsed imagePromptResp
		if err := json.Unmarshal(b, &parsed); err != nil {
			// fallback: just use the shot text
			parsed.NumImages = 1
			parsed.Images = []struct {
				Prompt         string  `json:"prompt"`
				NegativePrompt string  `json:"negative_prompt,omitempty"`
				Duration       float64 `json:"duration,omitempty"`
				Seed           *int    `json:"seed,omitempty"`
			}{
				{Prompt: strings.TrimSpace(shot.Text), Duration: round2(shot.End - shot.Start)},
			}
		}

		// Guardrails
		if parsed.NumImages != 1 || len(parsed.Images) != 1 {
			return nil, fmt.Errorf("LLM prompt count mismatch for shot %d: %+v", i, parsed.NumImages)
		}

		prompt := strings.TrimSpace(parsed.Images[0].Prompt)
		if prompt == "" {
			prompt = strings.TrimSpace(shot.Text)
		}

		// ---- Image generation (unchanged) ----
		urls, err := rs.GetImages(prompt, 1, mode)
		if err != nil {
			return nil, fmt.Errorf("image %d: %w", i, err)
		}
		if len(urls) == 0 || urls[0] == "" {
			return nil, fmt.Errorf("image %d: empty result", i)
		}

		// Lock duration to plan timing
		dur := shot.End - shot.Start
		out = append(out, models.ImageWithTimestamp{
			URL:       urls[0],
			Timestamp: dur,
		})
	}

	return out, nil
}

// ---- Helpers (same as before) ----

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

func shortHash(b []byte) string {
	h := sha1.Sum(b)
	return hex.EncodeToString(h[:8])
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
