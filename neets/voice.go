package neets

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type Neets struct {
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

func CreateNeets() *Neets {
	apiKey := os.Getenv("NEETS_API_KEY")
	if apiKey == "" {
		log.Fatalf("NEETS_API_KEY not set in .env file")
	}

	return &Neets{apiKey: apiKey}
}

func (n *Neets) NewVoiceRequest(text string, voiceId string) *VoiceRequest {
	p := RequestParams{
		Model: "style-diff-500",
	}
	request := VoiceRequest{
		Text:    text,
		VoiceId: voiceId,
		Params:  p,
		URL:     "https://api.neets.ai/v1/tts",
		ApiKey:  n.apiKey,
	}

	return &request
}

func (vr *VoiceRequest) Call(path string) (string, error) {
	body := []byte(
		fmt.Sprintf(`{
		"text": "%s",
		"voice_id": "%s",
		"params":{
		"model": "%s"
		}
		}`, vr.Text, vr.VoiceId, vr.Params.Model),
	)

	r, err := http.NewRequest("POST", vr.URL, bytes.NewBuffer(body))

	if err != nil {
		return "", errors.New("Error when trying to build a voice request")
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("X-API-Key", vr.ApiKey)

	client := &http.Client{}
	res, err := client.Do(r)

	if err != nil {
		return "", fmt.Errorf("%v", err)
	}

	defer res.Body.Close()

	file, err := io.ReadAll(res.Body)

	if err != nil {
		return "", errors.New("Error when trying to read a voice response")
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Error: received non-200 response code: %v", res)
	}

	if res.Header.Get("Content-Type") != "audio/mpeg" {
		log.Printf("Unexpected content type: %s", res.Header.Get("Content-Type"))
		log.Printf("Response body: %s", string(file))
		return "", errors.New("Error: response is not an MP3 file")
	}

	if err := os.WriteFile(path, file, 0644); err != nil {
		return "", fmt.Errorf("Error during writing data: %v", err)
	}

	return path, nil

}

func GetVoice() {
	// Placeholder function for future implementation
}
