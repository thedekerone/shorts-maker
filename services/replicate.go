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

func (rs *ReplicateService) GetCompletition(prompt string) (*models.PredictionOutputFormat, error) {
	ctx := context.TODO()
	model := "meta/meta-llama-3-70b-instruct:fbfb20b472b2f3bdd101412a9f70a0ed4fc0ced78a77ff00970ee7a2383c575d"

	input := replicate.PredictionInput{
		"system_prompt": "You are a tiktok script writer, you need to generate interesting stories for your audience. respond in the json format: {script: string; tags: string[]}. return only the json, don't include any other text. stories should last aproximately 3 minutes. this will be sent to a text to speech service to generate the final video, add punctuation and capitalization to make the text sound more natural.",
		"prompt":        prompt,
	}

	output, err := rs.client.Run(ctx, model, input, nil)

	if err != nil {
		return nil, err
	}

	if output == nil {
		return nil, errors.New("output is nil")
	}

	stringOutput := outputToString(output)

	test := models.PredictionOutputFormat{}
	json.Unmarshal([]byte(stringOutput), &test)

	return &test, nil

}

func outputToString[T any](output T) string {
	switch v := any(output).(type) {
	case []any:
		stringOutput := make([]string, len(v))
		for i, item := range v {
			if str, ok := item.(string); ok {
				stringOutput[i] = str
			} else {
				return ""
			}
		}
		return strings.Join(stringOutput, "")
	case string:
		return v
	default:
		return ""
	}
}
