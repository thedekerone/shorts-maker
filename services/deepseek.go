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

type ClipPromptGenerator struct {
	NumClips int `json:"numClips"`
	Clips    []struct {
		Prompt   string `json:"prompt"`
		Duration int    `json:"duration"` // must be 5, 6, 7, or 8
	} `json:"clips"`
}

type DeepSeekService struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

type DeepSeekRequest struct {
	MaxTokens   int       `json:"max_tokens"`
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
	ID      string `json:"id"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Index        int    `json:"index"`
		Message      struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			ToolCalls        []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
			Role string `json:"role"`
		} `json:"message"`
		Logprobs struct {
			Content []struct {
				Token       string  `json:"token"`
				Logprob     float64 `json:"logprob"`
				Bytes       []int   `json:"bytes"`
				TopLogprobs []struct {
					Token   string  `json:"token"`
					Logprob float64 `json:"logprob"`
					Bytes   []int   `json:"bytes"`
				} `json:"top_logprobs"`
			} `json:"content"`
		} `json:"logprobs"`
	} `json:"choices"`
	Created           int    `json:"created"`
	Model             string `json:"model"`
	SystemFingerprint string `json:"system_fingerprint"`
	Object            string `json:"object"`
	Usage             struct {
		CompletionTokens        int `json:"completion_tokens"`
		PromptTokens            int `json:"prompt_tokens"`
		PromptCacheHitTokens    int `json:"prompt_cache_hit_tokens"`
		PromptCacheMissTokens   int `json:"prompt_cache_miss_tokens"`
		TotalTokens             int `json:"total_tokens"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
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
	reqBody := DeepSeekRequest{
		Model:     "deepseek-reasoner",
		MaxTokens: 10000,
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

	fmt.Printf("\n\n\n%v", body)

	var apiResponse DeepSeekResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {

		fmt.Printf("\n ERROR UNMARHSALLING")
		return nil, err
	}

	fmt.Printf("=========================\n")
	fmt.Printf("\n\n\n%v", apiResponse.Choices[0].Message.Content)

	if len(apiResponse.Choices) == 0 {
		return nil, errors.New("no completions returned")
	}

	var jsonResponse ImagePromptGenerator
	if err := json.Unmarshal([]byte(apiResponse.Choices[0].Message.Content), &jsonResponse); err != nil {
		return nil, err
	}

	return &jsonResponse, nil
}

func (ds *DeepSeekService) GetCompletitionForClips(prompt, systemPrompt string) (*ClipPromptGenerator, error) {
	reqBody := DeepSeekRequest{
		Model:     "deepseek-reasoner",
		MaxTokens: 8192,
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

	var apiResp DeepSeekResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	if len(apiResp.Choices) == 0 {
		return nil, errors.New("no completions returned")
	}

	// The assistant’s JSON for the clips is inside Choices[0].Message.Content
	var clips ClipPromptGenerator
	if err := json.Unmarshal([]byte(apiResp.Choices[0].Message.Content), &clips); err != nil {
		return nil, err
	}

	return &clips, nil
}
