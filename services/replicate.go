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

func (rs *ReplicateService) GetCompletition(prompt string, systemPrompt string) (string, error) {
	ctx := context.TODO()
	model := "meta/meta-llama-3-70b-instruct:fbfb20b472b2f3bdd101412a9f70a0ed4fc0ced78a77ff00970ee7a2383c575d"

	if systemPrompt == "" {
		systemPrompt = `
		You are a creative storytelling AI designed to generate engaging, you create stories on the same language as the input, short-form stories suitable for TikTok's text-to-speech feature. Your task is to create captivating stories based on simple text prompts.
Guidelines:

Generate a story based on the given text prompt.
Keep the story concise, aiming for 60-120 seconds when read aloud.
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
	}

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

	return strings.Join(stringOutput, ""), nil

}

func (rs *ReplicateService) GetImages(prompt string, quantity int64) ([]string, error) {
	ctx := context.TODO()
	model := "black-forest-labs/flux-schnell"

	input := replicate.PredictionInput{
		"prompt":                 prompt,
		"num_outputs":            quantity,
		"disable_safety_checker": true,
		"aspect_ratio":           "9:16",
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
	model := "victor-upmeet/whisperx:84d2ad2d6194fe98a17d2b60bef1c7f910c46b2f6fd38996ca457afd9c8abfcb"

	input := replicate.PredictionInput{
		"audio_file":     audio,
		"align_output":   true,
		"batch_size":     128,
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
