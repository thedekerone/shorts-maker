package elevenlabs

import (
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

func CreateEleven() *Eleven {
	apiKey := os.Getenv("ELEVENLABS_API_KEY")
	if apiKey == "" {
		log.Fatalf("ELEVENLABS_API_KEY not set in .env file")
	}

	return &Eleven{apiKey: apiKey}
}

func (n *Eleven) NewVoiceRequest(text string, voiceId string) *VoiceRequest {
	//9BWtsMINqrJLrRacOk9x
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

func (vr *VoiceRequest) Call(path string) (string, error) {
	payload := strings.NewReader(fmt.Sprintf("{\n  \"text\": \"%s\"\n}", vr.Text))

	r, err := http.NewRequest("POST", vr.URL+"/"+vr.VoiceId, payload)

	println(vr.URL + "/" + vr.VoiceId)

	if err != nil {
		return "", errors.New("Error when trying to build a voice request")
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("xi-api-key", vr.ApiKey)

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
