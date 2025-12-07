package images

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/thedekerone/shorts-maker/elevenlabs"
	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/prompts"
	"github.com/thedekerone/shorts-maker/services"
)

func GetImagesWithTimestamps(shotPlan *elevenlabs.ShotPlan, mode string, userStyle string) (*models.ImagePlan, error) {
	promptSvc, err := services.NewPromptService()
	if err != nil {
		return nil, fmt.Errorf("create prompt service: %w", err)
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

	userPrompt := "INPUT:\n" + string(inputJSON)
	systemPrompt := prompts.ImageSystemPrompt()

	promptResults, err := promptSvc.GenerateImagePlan(userPrompt, systemPrompt)
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

	basePrompts := alignImagePrompts(promptResults.ImagesPrompt, shotPlan.Shots)
	linkedPrompts := linkPromptsWithContext(basePrompts, shotPlan.Shots)
	jobs := make([]imageJob, len(linkedPrompts))
	for i, prompt := range linkedPrompts {
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

			variants := buildPromptVariants(stylePrefix, job.prompt, job.shot)
			var (
				imagePath  string
				promptUsed string
				lastErr    error
			)

			for attempt, candidate := range variants {
				urls, err := rs.GetImages(candidate, 1, mode)
				if err != nil {
					lastErr = err
					log.Printf("image job %d attempt %d failed: %v", job.index, attempt+1, err)
					continue
				}
				if len(urls) == 0 || urls[0] == "" {
					lastErr = fmt.Errorf("empty output")
					log.Printf("image job %d attempt %d returned empty output", job.index, attempt+1)
					continue
				}

				imagePath = urls[0]
				promptUsed = candidate
				break
			}

			if imagePath == "" {
				once.Do(func() {
					if lastErr != nil {
						firstErr = fmt.Errorf("image %d failed after retries: %w", job.index, lastErr)
					} else {
						firstErr = fmt.Errorf("image %d failed after retries", job.index)
					}
				})
				return
			}

			dur := job.shot.End - job.shot.Start
			out[job.index] = models.ImageWithTimestamp{
				URL:       imagePath,
				Timestamp: dur,
				Prompt:    promptUsed,
			}
		}(job)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	return &models.ImagePlan{StylePrompt: styleAnchor, Shots: out}, nil
}

const safePromptSuffix = " Ensure scene remains fully clothed, public-friendly, PG-rated; no gore, nudity, or explicit content."

func alignImagePrompts(entries []services.ImagesPrompts, shots []elevenlabs.Shot) []string {
	prompts := make([]string, len(shots))
	for i := range shots {
		if i < len(entries) {
			prompts[i] = strings.TrimSpace(entries[i].Prompt)
		}
		if prompts[i] == "" {
			prompts[i] = fallbackPromptFromShot(shots[i])
		}
	}
	if len(entries) != len(shots) {
		log.Printf("LLM prompt count mismatch: got %d for %d shots", len(entries), len(shots))
	}
	return prompts
}

func linkPromptsWithContext(prompts []string, shots []elevenlabs.Shot) []string {
	linked := make([]string, len(prompts))
	for i, prompt := range prompts {
		trimmed := strings.TrimSpace(prompt)
		if i == 0 {
			linked[i] = fmt.Sprintf("OPENING SHOT: %s", trimmed)
			continue
		}

		prev := "previous beat"
		if i-1 < len(shots) {
			prev = sanitizeShotText(shots[i-1].Text)
		}
		curr := "current beat"
		if i < len(shots) {
			curr = sanitizeShotText(shots[i].Text)
		}

		linked[i] = fmt.Sprintf("CONTINUATION from previous moment (%s) into current action (%s). %s", prev, curr, trimmed)
	}
	return linked
}

func fallbackPromptFromShot(shot elevenlabs.Shot) string {
	sanitized := sanitizeShotText(shot.Text)
	return fmt.Sprintf("SCENE: %s. NEGATIVES: nudity, gore, explicit content, watermark, blur.", sanitized)
}

func buildPromptVariants(stylePrefix string, basePrompt string, shot elevenlabs.Shot) []string {
	basePrompt = strings.TrimSpace(basePrompt)
	if basePrompt == "" {
		basePrompt = strings.TrimSpace(shot.Text)
	}

	sanitized := sanitizeShotText(shot.Text)

	return []string{
		fmt.Sprintf("%s\n\n %s%s", stylePrefix, basePrompt, safePromptSuffix),
		fmt.Sprintf("%s\n\n SCENE: %s. NEGATIVES: nudity, lingerie, gore, explicit content, graphic violence.%s", stylePrefix, sanitized, safePromptSuffix),
		fmt.Sprintf("%s\n\n SCENE: cinematic establishing shot of %s, crowd-friendly. NEGATIVES: nudity, blood, injuries, suggestive poses.%s", stylePrefix, sanitized, safePromptSuffix),
	}
}

func sanitizeShotText(text string) string {
	clean := strings.ReplaceAll(text, "\n", " ")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return "the scene's environment"
	}
	if len(clean) > 160 {
		clean = clean[:160]
	}
	return clean
}
