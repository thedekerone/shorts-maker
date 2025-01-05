package engine_test

import (
	"context"
	"testing"

	"github.com/thedekerone/shorts-maker/engine"
	"github.com/thedekerone/shorts-maker/subtitles"
)

func TestRenderText(t *testing.T) {
	text := "this is an example of a test with subtitles la ala all ala l"

	err := engine.RenderText(text, "testsImages", "image_test.png")

	if err != nil {
		t.Log(err)
		t.Fatalf("Failed to run render text")
	}

	t.Log("good")
}

func skipTestSubtitleImage(t *testing.T) {
	text := "this is a test for subtitle images 22222 dsa das asddsa asdsda saddsa dasdsa dsa  dsaads "

	subs := subtitles.Subtitle{
		Text:      text,
		StartTime: 1.0,
		EndTime:   2.0,
	}
	image, err := subtitles.CreateSubtitleImage(&subs, "testsImages")

	if err != nil {
		t.Log(err)
		t.Fatalf("failed to render sub")
	}

	t.Log(image)
	t.Log("good")
}

func skipTestAddSubsToImage(t *testing.T) {
	text := "this is a test for subtitle images 22222 dsa das asddsa asdsda saddsa dasdsa dsa  dsaads "

	subs := subtitles.Subtitle{
		Text:      text,
		StartTime: 1.0,
		EndTime:   2.0,
	}
	image, err := subtitles.CreateSubtitleImage(&subs, "testsImages")

	if err != nil {
		t.Log(err)
		t.Fatalf("failed to render sub")
	}
	ctx := context.Background()

	result, err := engine.AddSubtitlesToVideo(ctx, "../input.mp4", []engine.SubtitleImage{image}, "output.mp4")

	if err != nil {
		t.Log(err)
		t.Fatalf("failed to render sub")
	}

	print(result)

}
