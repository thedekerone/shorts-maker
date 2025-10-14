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

// ---- Types ----

type inShot struct {
	Index int     `json:"index"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type modelInput struct {
	TotalDuration float64  `json:"total_duration"`
	Shots         []inShot `json:"shots"`
}

type contextResp struct {
	StyleAnchor string   `json:"style_anchor"`
	Characters  []string `json:"characters"`
}

type perShotResp struct {
	Prompt   string  `json:"prompt"`
	Duration float64 `json:"duration"`
}

// ---- Public API ----

func GetImagesWithTimestamps(shotPlan *elevenlabs.ShotPlan, mode string) ([]models.ImageWithTimestamp, error) {
	if shotPlan == nil || len(shotPlan.Shots) == 0 {
		return nil, fmt.Errorf("empty shot plan")
	}

	// one client does both: completion + image gen
	rs, err := services.NewReplicateService()
	if err != nil {
		return nil, fmt.Errorf("create replicate service: %w", err)
	}

	// Build clean JSON input for the model
	in := modelInput{
		TotalDuration: shotPlan.TotalEnd,
		Shots:         make([]inShot, len(shotPlan.Shots)),
	}
	for i, s := range shotPlan.Shots {
		in.Shots[i] = inShot{Index: i, Start: s.Start, End: s.End, Text: strings.TrimSpace(s.Text)}
	}
	inputJSON, _ := json.Marshal(in)
	inputHash := shortHash(inputJSON)

	// ---- 1) Build context once ----
	ctx, err := buildContext(rs, inputJSON, inputHash)
	if err != nil {
		return nil, err
	}

	// ---- 2) Generate one prompt per shot (strict per-shot completion) ----
	out := make([]models.ImageWithTimestamp, 0, len(in.Shots))

	for i, shot := range in.Shots {
		// shot-specific user payload that includes origin + context
		userPayload := map[string]any{
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
			// Hard duration lock so the model can't drift:
			"strict_duration": round2(shot.End - shot.Start),
		}
		ujson, _ := json.Marshal(userPayload)

		perShotSystem := `You are an Image-Prompt Composer.

Return STRICT JSON ONLY for this ONE shot.

Follow these rules:
- Use the provided context (style_anchor + characters) verbatim to keep continuity.
- Do NOT invent new recurring characters or change immutable traits.
- Keep each prompt ≤ 70 words, concise and parsable.
- No camera tech (no lens/focal/aperture/ISO/shutter/DoF).
- Repeat the NEGATIVES exactly from style_anchor if present.
- Duration MUST equal the provided "strict_duration" (float).

Output JSON schema:
{
  "prompt": "<STYLE ANCHOR: …> <CHARACTERS: …> <SCENE: …> <NEGATIVES: …>",
  "duration": <float>
}`

		userPrompt := "SHOT_INPUT:\n" + string(ujson)

		raw, err := rs.GetCompletition(userPrompt, perShotSystem)
		if err != nil {
			return nil, fmt.Errorf("completion (shot %d) error: %w", i, err)
		}
		raw = stripJSON(raw)

		var pr perShotResp
		if err := json.Unmarshal([]byte(raw), &pr); err != nil {
			// degrade gracefully to the shot text if invalid JSON
			pr.Prompt = strings.TrimSpace(shot.Text)
			pr.Duration = round2(shot.End - shot.Start)
		}

		// Final guardrails
		if strings.TrimSpace(pr.Prompt) == "" {
			pr.Prompt = strings.TrimSpace(shot.Text)
		}
		// lock to plan timing
		pr.Duration = round2(shot.End - shot.Start)

		// ---- 3) Generate the image for this shot ----
		urls, err := rs.GetImages(pr.Prompt, 1, mode)
		if err != nil {
			return nil, fmt.Errorf("image %d: %w", i, err)
		}
		if len(urls) == 0 || urls[0] == "" {
			return nil, fmt.Errorf("image %d: empty result", i)
		}

		out = append(out, models.ImageWithTimestamp{
			URL:       urls[0],
			Timestamp: pr.Duration, // NOTE: field name is Timestamp but we pass duration (per your comment)
		})
	}

	return out, nil
}

// ---- Helpers ----

// buildContext makes a single completion that derives the global Style Anchor and Character Sheets,
// then returns them so we can reuse across per-shot completions.
func buildContext(rs *services.ReplicateService, inputJSON []byte, inputHash string) (*contextResp, error) {
	system := `You are an Image-Prompt Context Builder.

Task: From the full shot list (JSON), extract the global STYLE ANCHOR and CHARACTER SHEETS ONLY.
No prompts per shot here—just the shared context used by later steps.

Rules:
- STYLE ANCHOR must include: art direction, style family (+short reason), color palette, lighting ethos, aspect ratio (16:9 unless story demands otherwise), texture, negatives.
- CHARACTER SHEETS: one string per recurring subject with immutable, fixed wording (name, age, ethnicity, facial structure, hair, eyes, build, wardrobe items/colors, signature props).
- Return STRICT JSON only.

Output JSON schema:
{
  "style_anchor": "<text>",
  "characters": ["<fixed character sheet #1>", "<fixed character sheet #2>", ...]
}`

	user := map[string]any{
		"origin": map[string]any{
			"package":    "images",
			"function":   "GetImagesWithTimestamps",
			"stage":      "context_build",
			"input_hash": inputHash,
		},
		"input": json.RawMessage(inputJSON),
	}
	uj, _ := json.Marshal(user)

	raw, err := rs.GetCompletition("INPUT:\n"+string(uj), system)
	if err != nil {
		return nil, fmt.Errorf("context completion error: %w", err)
	}
	raw = stripJSON(raw)

	var ctx contextResp
	if err := json.Unmarshal([]byte(raw), &ctx); err != nil {
		return nil, fmt.Errorf("context JSON invalid: %w\nraw: %s", err, raw)
	}
	if strings.TrimSpace(ctx.StyleAnchor) == "" {
		return nil, fmt.Errorf("context missing style_anchor")
	}
	return &ctx, nil
}

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
