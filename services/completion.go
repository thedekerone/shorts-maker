// services/completion.go
package services

type ImagePromptCompleter interface {
	// Complete must return STRICT JSON (no fences/comments), or an error.
	Complete(systemPrompt, userPrompt string) (string, error)
}
