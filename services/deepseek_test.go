package services_test

import (
	"testing"

	"github.com/thedekerone/shorts-maker/services"
)

func TestGetCompletitionForImages_Success(t *testing.T) {
	service, err := services.NewDeepSeekService()

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)

	}
	result, err := service.GetCompletitionForImages("this is a test prompt, just write an example without overthinking", "")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.NumImages != 1 {
		t.Errorf("Expected NumImages to be 1, got %d", result.NumImages)
	}
	if len(result.ImagesPrompt) != 1 || result.ImagesPrompt[0].Prompt != "test" {
		t.Errorf("Expected image prompt 'test', got %v", result.ImagesPrompt)
	}
	t.Log(result)
}
