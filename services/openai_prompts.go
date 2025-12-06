package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

type ClipPromptGenerator struct {
	NumClips int `json:"numClips"`
	Clips    []struct {
		Prompt   string `json:"prompt"`
		Duration int    `json:"duration"`
	} `json:"clips"`
}

type OpenAIService struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

type openAIRequest struct {
	Model       string               `json:"model"`
	Messages    []openAIMessage      `json:"messages"`
	Temperature float64              `json:"temperature,omitempty"`
	Response    openAIResponseFormat `json:"response_format,omitempty"`
}

type openAIResponseFormat struct {
	Type string `json:"type"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
}

func NewPromptService() (*OpenAIService, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY environment variable not set")
	}

	model := os.Getenv("OPENAI_COMPLETIONS_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	return &OpenAIService{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1/chat/completions",
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func (s *OpenAIService) GenerateImagePlan(userPrompt, systemPrompt string) (*ImagePromptGenerator, error) {
	content, err := s.chatCompletion(systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	var plan ImagePromptGenerator
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, fmt.Errorf("decode image plan: %w", err)
	}
	return &plan, nil
}

func (s *OpenAIService) GenerateClipPlan(userPrompt, systemPrompt string) (*ClipPromptGenerator, error) {
	content, err := s.chatCompletion(systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	var plan ClipPromptGenerator
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, fmt.Errorf("decode clip plan: %w", err)
	}
	return &plan, nil
}

func (s *OpenAIService) chatCompletion(systemPrompt, userPrompt string) (string, error) {
	reqBody := openAIRequest{
		Model: s.model,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.2,
		Response:    openAIResponseFormat{Type: "json_object"},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal openai request: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, s.baseURL, bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai api request failed with status %d", resp.StatusCode)
	}

	var completion openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return "", fmt.Errorf("decode openai response: %w", err)
	}

	if len(completion.Choices) == 0 {
		return "", errors.New("openai response contains no choices")
	}

	return completion.Choices[0].Message.Content, nil
}
