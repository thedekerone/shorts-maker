package elevenlabs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type Eleven struct {
	apiKey string
}

type VoiceRequest struct {
	Text    string
	VoiceId string
	Params  RequestParams
	URL     string
	ApiKey  string
}

type RequestParams struct {
	Model string
}

// Request body structure according to ElevenLabs API
type TextToSpeechRequest struct {
	Text  string        `json:"text"`
	Model string        `json:"model_id"`
	Voice VoiceSettings `json:"voice_settings,omitempty"`
}

type VoiceSettings struct {
	Stability       float64 `json:"stability,omitempty"`
	SimilarityBoost float64 `json:"similarity_boost,omitempty"`
}

func CreateEleven() *Eleven {
	apiKey := os.Getenv("ELEVENLABS_API_KEY")
	if apiKey == "" {
		log.Fatalf("ELEVENLABS_API_KEY not set in .env file")
	}
	return &Eleven{apiKey: apiKey}
}

func (n *Eleven) NewVoiceRequest(text string, voiceId string) *VoiceRequest {
	p := RequestParams{
		Model: "eleven_monolingual_v1",
	}
	request := VoiceRequest{
		Text:    text,
		VoiceId: voiceId,
		Params:  p,
		URL:     "https://api.elevenlabs.io/v1/text-to-speech",
		ApiKey:  n.apiKey,
	}
	return &request
}

func (n *Eleven) NewVoiceRequestMultilingual(text string, voiceId string) *VoiceRequest {
	p := RequestParams{
		Model: "eleven_multilingual_v2",
	}
	request := VoiceRequest{
		Text:    text,
		VoiceId: voiceId,
		Params:  p,
		URL:     "https://api.elevenlabs.io/v1/text-to-speech",
		ApiKey:  n.apiKey,
	}
	return &request
}

func (vr *VoiceRequest) Call(path string) (string, error) {
	// Prepare the request body according to API specifications
	requestBody := TextToSpeechRequest{
		Text:  vr.Text,
		Model: vr.Params.Model,
		Voice: VoiceSettings{
			Stability:       0.5,  // Default value, adjust as needed
			SimilarityBoost: 0.75, // Default value, adjust as needed
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	// Create request with correct URL format
	url := fmt.Sprintf("%s/%s", vr.URL, vr.VoiceId)
	r, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("xi-api-key", vr.ApiKey)

	// Make the request
	client := &http.Client{}
	res, err := client.Do(r)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer res.Body.Close()

	// Read response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	// Handle non-200 responses
	if res.StatusCode != http.StatusOK {
		// Try to parse error message if available
		var errorResponse struct {
			Detail string `json:"detail"`
		}
		if err := json.Unmarshal(body, &errorResponse); err == nil && errorResponse.Detail != "" {
			return "", fmt.Errorf("API error (status %d): %s", res.StatusCode, errorResponse.Detail)
		}
		return "", fmt.Errorf("API error (status %d): %s", res.StatusCode, string(body))
	}

	// Verify content type
	if !strings.Contains(res.Header.Get("Content-Type"), "audio/mpeg") {
		log.Printf("Unexpected content type: %s", res.Header.Get("Content-Type"))
		log.Printf("Response body: %s", string(body))
		return "", errors.New("response is not an MP3 file")
	}

	// Write the audio file
	if err := os.WriteFile(path, body, 0644); err != nil {
		return "", fmt.Errorf("error writing file: %v", err)
	}

	return path, nil
}
