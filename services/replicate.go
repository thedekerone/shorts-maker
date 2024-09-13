package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/replicate/replicate-go"
	"github.com/thedekerone/shorts-maker/models"
)

type ReplicateService struct {
	Client *replicate.Client
}

func NewReplicateService() (*ReplicateService, error) {
	client, err := replicate.NewClient(replicate.WithTokenFromEnv())
	if err != nil {
		return nil, err
	}
	return &ReplicateService{Client: client}, nil
}

func (rs *ReplicateService) GetCompletition(prompt string) (string, error) {
	ctx := context.TODO()
	model := "meta/llama-2-70b-chat:2d19859030ff705a87c746f7e96eea03aefb71f166725aee39692f1476566d48"

	const systemPrompt = `
		You are a creative storytelling AI designed to generate engaging, short-form stories suitable for TikTok's text-to-speech feature. Your task is to create captivating stories based on simple text prompts.
Guidelines:

Generate a story based on the given text prompt.
Keep the story concise, aiming for 100-150 words (30-60 seconds when read aloud).
Use vivid, descriptive language to engage the listener.
Ensure the story has a clear beginning, middle, and end.
Incorporate elements of surprise, humor, or emotional appeal when appropriate.
Use simple language and short sentences for easy listening.
Avoid explicit content, excessive violence, or controversial topics.
End with a hook or twist to encourage engagement.
The story can be either real (based on historical events or facts) or fictional, depending on the prompt.
Adapt your storytelling style to best fit the prompt.

Input:
[Text prompt]
Output:
[Generated story text only]
Remember to generate only the story text, without any additional elements like titles or hashtags. Create a story that would be engaging and suitable for TikTok's audience.
		`

	input := replicate.PredictionInput{
		"system_prompt": systemPrompt,
		"prompt":        prompt,
	}

	output, err := rs.Client.Run(ctx, model, input, nil)

	if err != nil {
		return "", err
	}

	if output == nil {
		return "", errors.New("output is nil")
	}

	stringOutput := outputToStrings(output)

	return strings.Join(stringOutput, " "), nil

}

func (rs *ReplicateService) GetImages(prompt string, quantity int64) ([]string, error) {
	ctx := context.TODO()
	model := "black-forest-labs/flux-dev"

	input := replicate.PredictionInput{
		"prompt":                 prompt,
		"num_outputs":            quantity,
		"disable_safety_checker": true,
	}

	output, err := rs.RunWithModel(ctx, model, input, nil)

	if err != nil {
		return nil, err
	}

	if output == nil {
		return nil, errors.New("output is nil")
	}

	stringsOutput := outputToStrings(output)

	return stringsOutput, nil
}

func (rs *ReplicateService) GetVoice(text string) (string, error) {
	ctx := context.TODO()
	model := "lucataco/xtts-v2:49ff6cfa14bd4e7f80f62e2279f82f23dfc2e7970f825f8db5599f8a6213c009"

	input := replicate.PredictionInput{
		"text":    text,
		"speaker": "https://replicate.delivery/pbxt/KMZ6fyOMKrtwERmDWAJnd5KRy39a86dgloX7SYP5dVTnQXjv/jacob.wav",
	}

	output, err := rs.Client.Run(ctx, model, input, nil)

	if err != nil {
		return "", err
	}

	if output == nil {
		return "", errors.New("output is nil")
	}

	stringOutput := outputToStrings(output)

	return strings.Join(stringOutput, ""), nil
}

//get transcription

func (rs *ReplicateService) GetTranscription(audio string, initial string) (*models.TranscriptionOutput, error) {
	ctx := context.TODO()
	model := "thomasmol/whisper-diarization:aae6db69a923a6eab6bc3ec098148a8c9c999685be89f428a4a6072fca544d26"

	input := replicate.PredictionInput{
		"file":           audio,
		"language":       "en",
		"num_speakers":   1,
		"group_segments": false,
		"align_output":   true,
		"offset_seconds": 0,
	}

	output, err := rs.Client.Run(ctx, model, input, nil)

	if err != nil {
		println(err.Error())
		return nil, err
	}

	if output == nil {
		println("output is nil")
		return nil, errors.New("output is nil")
	}

	var formattedOutput models.TranscriptionOutput

	outputMap, ok := output.(map[string]interface{})
	if !ok {
		return nil, errors.New("output is not of type map[string]interface{}")
	}

	jsonData, err := json.Marshal(outputMap)
	if err != nil {
		println(err.Error())
		return nil, err
	}

	err = json.Unmarshal(jsonData, &formattedOutput)
	if err != nil {
		println(err.Error())
		return nil, err
	}

	return &formattedOutput, nil
}

func outputToStrings[T any](output T) []string {
	switch v := any(output).(type) {
	case []any:
		stringOutput := make([]string, len(v))
		for i, item := range v {
			if str, ok := item.(string); ok {
				stringOutput[i] = str
			} else {
				return nil
			}
		}
		return stringOutput
	case string:
		return []string{v}
	default:
		return nil
	}
}

func (rs *ReplicateService) RunWithModel(ctx context.Context, identifier string, input replicate.PredictionInput, webhook *replicate.Webhook) (replicate.PredictionOutput, error) {
	id, err := replicate.ParseIdentifier(identifier)

	prediction, err := rs.Client.CreatePredictionWithModel(ctx, id.Owner, id.Name, input, nil, false)

	if err != nil {
		return nil, err
	}

	err = rs.Client.Wait(ctx, prediction)

	if err != nil {
		return nil, err
	}

	return prediction.Output, nil
}

func (rs *ReplicateService) GetVoiceLarge(prompt string) ([]string, error) {
	const maxTokens = 600 // Adjust this value based on your specific requirements
	var result []string

	words := strings.Fields(prompt)
	var currentChunk []string
	var tokenCount int

	for _, word := range words {
		wordTokens := estimateTokens(word)
		if tokenCount+wordTokens > maxTokens && len(currentChunk) > 0 {
			voice, err := rs.GetVoice(strings.Join(currentChunk, " "))
			if err != nil {
				return nil, err
			}
			result = append(result, voice)
			currentChunk = nil
			tokenCount = 0
		}

		currentChunk = append(currentChunk, word)
		tokenCount += wordTokens
	}

	if len(currentChunk) > 0 {
		voice, err := rs.GetVoice(strings.Join(currentChunk, " "))
		if err != nil {
			return nil, err
		}
		result = append(result, voice)
	}

	return result, nil
}

// estimateTokens is a simple function to estimate the number of tokens in a word
// You may need to implement a more sophisticated tokenization method based on your specific requirements
func estimateTokens(word string) int {
	return len(word)/4 + 1 // A simple estimation, assuming on average 4 characters per token
}
