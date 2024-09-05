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
	client *replicate.Client
}

func NewReplicateService() (*ReplicateService, error) {
	client, err := replicate.NewClient(replicate.WithTokenFromEnv())
	if err != nil {
		return nil, err
	}
	return &ReplicateService{client: client}, nil
}

func (rs *ReplicateService) GetCompletition(prompt string) (string, error) {
	ctx := context.TODO()
	model := "meta/meta-llama-3-70b-instruct:fbfb20b472b2f3bdd101412a9f70a0ed4fc0ced78a77ff00970ee7a2383c575d"

	input := replicate.PredictionInput{
		"system_prompt": "You are a tiktok script writer assistant, you help to generate interesting stories for a writer. stories should last aproximately 3 minutes. don't add extra text, only include the story in your answer. only respond with the story, don use introduction or conclusion.",
		"prompt":        prompt,
	}

	output, err := rs.client.Run(ctx, model, input, nil)

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
	model := "black-forest-labs/flux-dev"

	input := replicate.PredictionInput{
		"prompt":      prompt,
		"num_outputs": quantity,
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
		"speaker": "http://velvetlettr.com/api/v1/download-shared-object/aHR0cDovLzEyNy4wLjAuMTo5MDAwL3JlcGxpY2F0ZS1maWxlcy9hdWRpby9FbGV2ZW5MYWJzXzIwMjQtMDktMDRUMThfNDBfMzRfQnJpYW5fcHJlX3M1MF9zYjc1X3NlMF9iX20yLm1wMz9YLUFtei1BbGdvcml0aG09QVdTNC1ITUFDLVNIQTI1NiZYLUFtei1DcmVkZW50aWFsPVZXNTJHOEkyMzVVSU40UFJHRVQzJTJGMjAyNDA5MDQlMkZ1cy1lYXN0LTElMkZzMyUyRmF3czRfcmVxdWVzdCZYLUFtei1EYXRlPTIwMjQwOTA0VDE4NDEyMlomWC1BbXotRXhwaXJlcz00MzIwMCZYLUFtei1TZWN1cml0eS1Ub2tlbj1leUpoYkdjaU9pSklVelV4TWlJc0luUjVjQ0k2SWtwWFZDSjkuZXlKaFkyTmxjM05MWlhraU9pSldWelV5UnpoSk1qTTFWVWxPTkZCU1IwVlVNeUlzSW1WNGNDSTZNVGN5TlRRNE9Ua3pOaXdpY0dGeVpXNTBJam9pYldsdWFXOWhaRzFwYmlKOS5DYmpZTXhVSHFyN2ZrdjZZOXpVbkhXVnZQdU9aNGh2b2FXcERuZ3BMZ0xMcVV0dEJqeGg5MUx4Y05PbjJqZ2djVThpYVZ2dmNiSzZ3Ul9IQ0p5WXZuQSZYLUFtei1TaWduZWRIZWFkZXJzPWhvc3QmdmVyc2lvbklkPW51bGwmWC1BbXotU2lnbmF0dXJlPWJmZTliYTJkZTdiMGU4MjQ2OTRjMGIwMjMzYWUyZTQzNjhhNzBjYWNlMDIyYTc1NThmYTRmNzRmZmM0MTdmNDI",
	}

	output, err := rs.client.Run(ctx, model, input, nil)

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
	model := "victor-upmeet/whisperx:77505c700514deed62ab3891c0011e307f905ee527458afc15de7d9e2a3034e8"

	input := replicate.PredictionInput{
		"audio_file":   audio,
		"align_output": true,
	}

	output, err := rs.client.Run(ctx, model, input, nil)

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

	prediction, err := rs.client.CreatePredictionWithModel(ctx, id.Owner, id.Name, input, nil, false)

	if err != nil {
		return nil, err
	}

	err = rs.client.Wait(ctx, prediction)

	if err != nil {
		return nil, err
	}

	return prediction.Output, nil
}
