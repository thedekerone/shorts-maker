package elevenlabs

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/thedekerone/shorts-maker/models"
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

type TTSWithTimestamps struct {
	Audio     string       `json:"audio_base64"`
	Alignment TTSAlignment `json:"alignment"`
}

type TTSAlignment struct {
	CharactersStartTimes []float64 `json:"character_start_times_seconds"`
	Characters           []string  `json:"characters"`
	CharactersEndTimes   []float64 `json:"character_end_times_seconds"`
}

type CallResponse struct {
	AudioPath     string
	Transcription *models.TranscriptionOutput
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

func (n *Eleven) NewVoiceRequestV3(text string, voiceId string) *VoiceRequest {
	p := RequestParams{
		Model: "eleven_v3",
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

func (vr *VoiceRequest) Call(path string, withTimestamps bool) (*CallResponse, error) {
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
		return nil, fmt.Errorf("error marshaling request: %v", err)
	}

	// Create request with correct URL format
	url := fmt.Sprintf("%s/%s", vr.URL, vr.VoiceId)

	if withTimestamps == true {
		url += fmt.Sprintf("/%s", "with-timestamps")
	}

	r, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("xi-api-key", vr.ApiKey)

	// Make the request
	client := &http.Client{}
	res, err := client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer res.Body.Close()

	// Read response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	// Handle non-200 responses
	if res.StatusCode != http.StatusOK {
		// Try to parse error message if available
		var errorResponse struct {
			Detail string `json:"detail"`
		}
		if err := json.Unmarshal(body, &errorResponse); err == nil && errorResponse.Detail != "" {
			return nil, fmt.Errorf("API error (status %d): %s", res.StatusCode, errorResponse.Detail)
		}
		return nil, fmt.Errorf("API error (status %d): %s", res.StatusCode, string(body))
	}

	if withTimestamps == true {
		var jsonBody TTSWithTimestamps

		json.Unmarshal(body, &jsonBody)

		dec, err := base64.StdEncoding.DecodeString(jsonBody.Audio)

		if err != nil {
			return nil, err
		}

		f, err := os.Create(path)

		if err != nil {
			return nil, err
		}

		if _, err := f.Write(dec); err != nil {
			return nil, err
		}

		if err := f.Sync(); err != nil {
			return nil, err
		}

		return &CallResponse{
			AudioPath:     path,
			Transcription: getTranscriptionOutput(&jsonBody.Alignment),
		}, nil
	}

	// Verify content type
	if !strings.Contains(res.Header.Get("Content-Type"), "audio/mpeg") {
		log.Printf("Unexpected content type: %s", res.Header.Get("Content-Type"))
		log.Printf("Response body: %s", string(body))
		return nil, errors.New("response is not an MP3 file")
	}

	// Write the audio file
	if err := os.WriteFile(path, body, 0644); err != nil {
		return nil, fmt.Errorf("error writing file: %v", err)
	}

	return &CallResponse{
		AudioPath:     path,
		Transcription: nil,
	}, nil
}

func getTranscriptionOutput(alignment *TTSAlignment) *models.TranscriptionOutput {
	if alignment == nil || len(alignment.Characters) == 0 {
		return &models.TranscriptionOutput{Segments: []models.Segment{}, Language: "en"}
	}

	chars := alignment.Characters
	starts := alignment.CharactersStartTimes
	ends := alignment.CharactersEndTimes

	n := min3(len(chars), len(starts), len(ends))
	if n == 0 {
		return &models.TranscriptionOutput{Segments: []models.Segment{}, Language: "en"}
	}

	isDigit := func(s string) bool {
		// fast-path ASCII digits; Characters should be single runes
		if len(s) != 1 {
			return false
		}
		r := []rune(s)[0]
		return r >= '0' && r <= '9'
	}

	// separators incl. comma & dot, but keep numeric tokens like 3.14 or 1,234 together
	isSep := func(i int) bool {
		s := chars[i]
		switch s {
		case " ", "\n", "\t", "—":
			return true
		case ".", ",":
			// don't split if between digits (decimals or thousands separators)
			if i > 0 && i+1 < n && isDigit(chars[i-1]) && isDigit(chars[i+1]) {
				return false
			}
			return true
		default:
			return false
		}
	}

	words := make([]models.Word, 0, n/3)
	var text strings.Builder
	text.Grow(n + n/3)

	var cur strings.Builder
	var curStart float64
	inWord := false

	flush := func(endIdx int) {
		if !inWord {
			return
		}
		w := models.Word{
			Word:  cur.String(),
			Start: curStart,
			End:   ends[endIdx],
		}
		words = append(words, w)
		if text.Len() > 0 {
			text.WriteByte(' ')
		}
		text.WriteString(w.Word)
		cur.Reset()
		inWord = false
	}

	for i := 0; i < n; i++ {
		if isSep(i) {
			if inWord {
				flush(i - 1) // end previous word at the last non-separator char
			}
			continue
		}
		if !inWord {
			curStart = starts[i]
			inWord = true
		}
		cur.WriteString(chars[i])
	}
	if inWord {
		flush(n - 1)
	}

	if len(words) == 0 {
		return &models.TranscriptionOutput{Segments: []models.Segment{}, Language: "en"}
	}

	segment := models.Segment{
		Text:  text.String(),
		Start: words[0].Start,
		End:   words[len(words)-1].End,
		Words: words,
	}

	return &models.TranscriptionOutput{
		Segments: []models.Segment{segment},
		Language: "en",
	}
}

func min3(a, b, c int) int {
	if a > b {
		a = b
	}
	if a > c {
		a = c
	}
	return a
}
