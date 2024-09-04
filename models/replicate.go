package models

type CompletitionRequest struct {
	Stream bool                     `json:"stream"`
	Input  CompletitionRequestInput `json:"input"`
}

type CompletitionRequestInput struct {
	TopP            float64 `json:"top_p"`
	Prompt          string  `json:"prompt"`
	MinTokens       int     `json:"min_tokens"`
	Temperature     float64 `json:"temperature"`
	PromptTemplate  string  `json:"prompt_template"`
	PresencePenalty float64 `json:"presence_penalty"`
}

type PollPredictionResponse struct {
	Id     string   `json:"id"`
	Output []string `json:"output"`
	Status string   `json:"status"`
	Urls   struct {
		Get    string `json:"get"`
		Cancel string `json:"cancel"`
	} `json:"urls"`
}

type PredictionOutputFormat struct {
	Script string   `json:"script"`
	Tags   []string `json:"tags"`
}
