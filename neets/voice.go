package neets

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

type Neets struct {
	apiKey string
}

type VoiceRequest struct {
	Text    string        `json:"text"`
	VoiceId string        `json:"voice_id"`
	Params  RequestParams `json:"params"`
	URL     string
	ApiKey  string
}

type RequestParams struct {
	Model string `json:"model"`
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
		Model: "ar-diff-50k",
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
	jsonBody, err := json.Marshal(vr)
	if err != nil {
		return "", errors.New("Error when marshalling json")
	}

	r, err := http.NewRequest("POST", vr.URL, bytes.NewBuffer(jsonBody))

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

func (vr *VoiceRequest) CallByChunks(path string) (string, error) {
	const chunkSize = 300
	chunks := splitTextIntoChunks(vr.Text, chunkSize)

	var combinedAudio []string

	for i, chunk := range chunks {
		vr.Text = chunk
		chunkPath := fmt.Sprintf("%s_chunk_%d.mp3", path, i)
		audioPath, err := vr.Call(chunkPath)
		if err != nil {
			return "", err
		}

		combinedAudio = append(combinedAudio, audioPath)
	}

	// Create a temporary file listing all chunk files
	listFile, err := os.CreateTemp("", "chunks_list_*.txt")
	if err != nil {
		return "", fmt.Errorf("Error creating temporary file: %v", err)
	}
	defer os.Remove(listFile.Name())

	for _, chunkPath := range combinedAudio {
		if _, err := listFile.WriteString(fmt.Sprintf("file '%s'\n", chunkPath)); err != nil {
			return "", fmt.Errorf("Error writing to temporary file: %v", err)
		}
	}
	listFile.Close()

	// Use ffmpeg to concatenate the audio files
	cmd := exec.Command("ffmpeg", "-f", "concat", "-safe", "0", "-i", listFile.Name(), "-c", "copy", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("Error running ffmpeg: %v", err)
	}

	// Clean up temporary chunk files
	for _, chunkPath := range combinedAudio {
		if err := os.Remove(chunkPath); err != nil {
			log.Printf("Warning: could not remove temporary chunk file %s: %v", chunkPath, err)
		}
	}

	return path, nil
}

func splitTextIntoChunks(text string, chunkSize int) []string {
	var chunks []string
	for len(text) > chunkSize {
		splitIndex := findSplitIndex(text[:chunkSize])
		chunks = append(chunks, text[:splitIndex])
		text = text[splitIndex:]
	}
	chunks = append(chunks, text)
	return chunks
}

func findSplitIndex(text string) int {
	lastLineBreak := strings.LastIndex(text, "\n")
	lastDot := strings.LastIndex(text, ".")
	if lastLineBreak == -1 && lastDot == -1 {
		return len(text)
	}
	if lastLineBreak > lastDot {
		return lastLineBreak + 1
	}
	return lastDot + 1
}

func GetVoice() {
	// Placeholder function for future implementation
}
