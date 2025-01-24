package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

type DeepSeekService struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

type DeepSeekRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type DeepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func NewDeepSeekService() (*DeepSeekService, error) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return nil, errors.New("DEEPSEEK_API_KEY environment variable not set")
	}

	return &DeepSeekService{
		APIKey:     apiKey,
		BaseURL:    "https://api.deepseek.com/chat/completions",
		HTTPClient: &http.Client{},
	}, nil
}

func (ds *DeepSeekService) GetCompletitionForImages(prompt string, systemPrompt string) (*ImagePromptGenerator, error) {

	if systemPrompt == "" {
		systemPrompt = `You are a image prompt generator, the user will show you a story and you have to return a JSON with the following format:
		{
			numImages: number,
			images: [
				{ prompt: "string", duration: number, segmentIndex: number }
			]
		}`
	}

	reqBody := DeepSeekRequest{
		Model: "deepseek-reasoner",
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
		Stream: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", ds.BaseURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ds.APIKey)

	resp, err := ds.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResponse DeepSeekResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, err
	}

	if len(apiResponse.Choices) == 0 {
		return nil, errors.New("no completions returned")
	}

	var jsonResponse ImagePromptGenerator
	if err := json.Unmarshal([]byte(apiResponse.Choices[0].Message.Content), &jsonResponse); err != nil {
		return nil, err
	}

	return &jsonResponse, nil
}
