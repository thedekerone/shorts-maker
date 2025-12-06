package images

import (
	"encoding/json"
	"fmt"
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
