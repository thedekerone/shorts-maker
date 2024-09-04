package services

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/thedekerone/shorts-maker/models"
)

type ReplicateService struct {
	apiKey string
}

func NewReplicateService(apiKey string) *ReplicateService {
	return &ReplicateService{apiKey: apiKey}
}

func (rs *ReplicateService) GetCompletition(prompt string, url string) string {
	replicateRequest := models.CompletitionRequest{
		Stream: false,
		Input: models.CompletitionRequestInput{
			TopP:            0.9,
			Prompt:          prompt,
			MinTokens:       50,
			Temperature:     0.9,
			PromptTemplate:  "",
			PresencePenalty: 0.0,
		},
	}

	resp, err := rs.makeRequest(url, "POST", replicateRequest)

	if err != nil {
		return err.Error()
	}

	defer resp.Body.Close()

	buf := new(bytes.Buffer)

	buf.ReadFrom(resp.Body)
	jsonResponse := models.PollPredictionResponse{}
	err = json.Unmarshal(buf.Bytes(), &jsonResponse)

	if err != nil {
		return err.Error()
	}

	time.Sleep(1 * time.Second)
	ppr := rs.PollPrediction(jsonResponse.Id)

	for ppr.Status != "completed" {
		ppr = rs.PollPrediction(jsonResponse.Id)
		time.Sleep(1 * time.Second)
	}

	return ppr.Output[0]
}

func (rs *ReplicateService) PollPrediction(id string) models.PollPredictionResponse {
	c, err := http.NewRequest("GET", "https://api.replicate.com/v1/prediction/"+id, nil)

	if err != nil {
		return models.PollPredictionResponse{}
	}

	c.Header.Set("Authorization", "Bearer "+rs.apiKey)

	client := &http.Client{}

	resp, err := client.Do(c)

	if err != nil {
		return models.PollPredictionResponse{}
	}

	ppr := models.PollPredictionResponse{}

	buf := new(bytes.Buffer)

	buf.ReadFrom(resp.Body)

	err = json.Unmarshal(buf.Bytes(), &ppr)

	if err != nil {
		return models.PollPredictionResponse{}
	}

	return ppr

}

func (rs *ReplicateService) makeRequest(url string, method string, request models.CompletitionRequest) (*http.Response, error) {
	body, err := json.Marshal(request)

	if err != nil {
		return nil, err
	}

	c, err := http.NewRequest(method, url, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	c.Header.Set("Authorization", "Bearer "+rs.apiKey)
	c.Header.Set("Content-Type", "application/json")
	c.Header.Set("Accept", "application/json")
	c.Header.Set("User-Agent", "Replicate API Client")

	client := &http.Client{}
	resp, err := client.Do(c)

	if err != nil {
		return nil, err
	}

	return resp, nil

}

func GetApiKeyFromEnv() string {
	return os.Getenv("REPLICATE_API_KEY")
}
