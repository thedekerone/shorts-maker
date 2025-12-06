package images

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/thedekerone/shorts-maker/elevenlabs"
	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/services"
)

type imagePromptResp struct {
	NumImages   int    `json:"numImages"`
	StylePrompt string `json:"stylePrompt"`
	Images      []struct {
		Prompt         string  `json:"prompt"`
		NegativePrompt string  `json:"negative_prompt,omitempty"`
		Duration       float64 `json:"duration,omitempty"` // ignored; we lock to ShotPlan
		Seed           *int    `json:"seed,omitempty"`
	} `json:"images"`
}

func GetImagesWithTimestamps(shotPlan *elevenlabs.ShotPlan, mode string, userStyle string) (*models.ImagePlan, error) {
	deepseek, err := services.NewDeepSeekService()
	if err != nil {
		return nil, fmt.Errorf("create deepseek service: %w", err)
	}

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

Each image is generated independently (no shared state). Keep visuals cohesive across images.

HARD RULES

Keep a consistent look across all images.

Use clear labeled sections inside each prompt; simple, declarative phrasing.

Avoid pronouns for recurring subjects; restate names and fixed traits when a character appears.

Output ONLY strict JSON (no markdown, no comments, no extra text).

INPUT FORMAT (from the user)
You will receive a list of shots with text, start, and end timestamps, and may also receive a total_duration. Example:
{ shot: "<text>", start: <float>, end: <float> }
{ shot: "<text>", start: <float>, end: <float> }
...

INSTRUCTIONS

Read all shots and infer the story’s mood & genre (e.g., hopeful, melancholic, tense; drama, thriller, adventure).

Choose a matching Style Family (e.g., “gritty neo-noir”, “warm nostalgic drama”, “cold techno-thriller”, “sun-bleached road movie”, “whimsical watercolor fable”).

Create a single STYLE ANCHOR and reuse it verbatim for all images. The anchor must be a single line beginning with:
STYLE: <medium/technique>, <palette>, <lighting>, <lens/framing>, <texture/grain>, <overall vibe>; negatives: <comma-separated negatives>
Examples:

STYLE: rough sketch drawing, soft pastel colors, overcast diffused light, 50mm framing, paper grain, intimate melancholic drama; negatives: overexposed, underexposed, blur, duplicate limbs, extra fingers, text, watermark, logo, frame text, gore

STYLE: photoreal 35mm film, muted teal-orange palette, golden hour rim light, shallow depth of field, subtle film grain, warm nostalgic road movie; negatives: overexposed, underexposed, posterization, CGI look, banding, text, watermark, logo

Shot planning & pacing

Produce exactly one image per input shot, in the same order.

If a shot provides start and end, set that image’s duration = end − start (round to 2 decimals).

If total_duration is provided and timestamps are missing, pace ~6–8s per image and ensure the sum of durations equals total_duration (adjust the final item to correct rounding drift).

Image prompting

For each image, decide what should be shown so the story makes sense:
• If a character is essential, include a brief, fixed description (age, visible traits, clothing colors, signature prop) in the SCENE text and repeat it consistently whenever that character reappears.
• If the moment is conceptual/abstract, depict clear, concrete visuals that communicate the idea (objects, environments, diagrams, symbols).

Write a stand-alone prompt with these labeled sections (keep ≤ 70 words total):
• SCENE: <setting, time of day, weather, key props, physical actions or abstract elements; objective description only; simple sentences>
• NEGATIVES: <repeat the negatives from the STYLE line verbatim>

Maintain the same camera language and palette implied by the STYLE line.

OUTPUT FORMAT (strict JSON only)
{
"numImages": <integer>,
"stylePrompt": "STYLE: <medium/technique>, <palette>, <lighting>, <lens/framing>, <texture/grain>, <overall vibe>; negatives: <comma-separated negatives>",
"images": [
{
"prompt": "<SCENE: …> <NEGATIVES: …>",
"duration": <float>
}
// one object per shot, in order
]
}

VALIDATION

numImages must equal the number of input shots.

If shot timestamps are present, each duration must equal end − start (rounded to two decimals).

If total_duration is provided, the sum of durations must equal total_duration (±0.01). Correct any rounding drift on the final item.

Strict mode option: If shot timestamps are present, do not change durations; use exactly end − start.

QUALITY CHECKS

A single STYLE line exists, begins with “STYLE:”, includes a “; negatives: …” list, and is reused verbatim across all images.

Each image prompt contains exactly two labeled sections: SCENE and NEGATIVES.

Wording is concise, objective, and, when characters recur, uses the same fixed descriptors every time.`

	userPrompt := "INPUT:\n" + string(inputJSON)

	promptResults, err := deepseek.GetCompletitionForImages(userPrompt, system)
	if err != nil {
		return nil, fmt.Errorf("completion error: %w", err)
	}

	if len(promptResults.ImagesPrompt) != len(shotPlan.Shots) {
		return nil, fmt.Errorf("LLM prompt count mismatch: got %d for %d shots", len(promptResults.ImagesPrompt), len(shotPlan.Shots))
	}

	styleAnchor := strings.TrimSpace(userStyle)
	if styleAnchor == "" {
		styleAnchor = strings.TrimSpace(promptResults.StylePrompt)
	}
	if styleAnchor == "" {
		styleAnchor = "STYLE: cinematic film, balanced palette, soft rim light, 35mm framing, subtle grain, modern short; negatives: blur, duplicate limbs, extra fingers, text, watermark"
	}

	stylePrefix := styleAnchor
	if !strings.HasSuffix(stylePrefix, ".") {
		stylePrefix += "."
	}

	const maxConcurrentImageJobs = 3

	type imageJob struct {
		index  int
		shot   elevenlabs.Shot
		prompt string
	}

	jobs := make([]imageJob, len(promptResults.ImagesPrompt))
	for i, p := range promptResults.ImagesPrompt {
		prompt := strings.TrimSpace(p.Prompt)
		if prompt == "" {
			prompt = strings.TrimSpace(shotPlan.Shots[i].Text)
		}
		jobs[i] = imageJob{index: i, shot: shotPlan.Shots[i], prompt: prompt}
	}

	out := make([]models.ImageWithTimestamp, len(jobs))

	var (
		wg       sync.WaitGroup
		sem      = make(chan struct{}, maxConcurrentImageJobs)
		once     sync.Once
		firstErr error
	)

	for _, job := range jobs {
		wg.Add(1)
		go func(job imageJob) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			finalPrompt := stylePrefix + "\n\n " + job.prompt
			urls, err := rs.GetImages(finalPrompt, 1, mode)
			if err != nil {
				once.Do(func() { firstErr = fmt.Errorf("image %d: %w", job.index, err) })
				return
			}
			if len(urls) == 0 || urls[0] == "" {
				once.Do(func() { firstErr = fmt.Errorf("image %d: empty result", job.index) })
				return
			}

			dur := job.shot.End - job.shot.Start
			out[job.index] = models.ImageWithTimestamp{
				URL:       urls[0],
				Timestamp: dur,
				Prompt:    finalPrompt,
			}
		}(job)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	return &models.ImagePlan{StylePrompt: styleAnchor, Shots: out}, nil
}
