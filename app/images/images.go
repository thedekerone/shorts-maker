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

	system := `You are an "Image Prompt Composer". Output STRICT JSON only:
{"numImages":int,"images":[{"prompt":string,"negative_prompt":string?}]}
Write exactly one prompt per input shot, same order. Keep consistent style across prompts. No camera-tech jargon. No markdown.`

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
