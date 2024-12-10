package engine_test

import (
	"testing"

	"github.com/thedekerone/shorts-maker/engine"
)

func TestRenderText(t *testing.T) {
	text := "this is an example of a\n test with subtitles"

	err := engine.RenderText(text, "testsImages", "image_test.png")

	if err != nil {
		t.Log(err)
		t.Fatalf("Failed to run render text")
	}

	t.Log("good")

}
