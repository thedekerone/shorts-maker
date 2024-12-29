package subtitles_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/thedekerone/shorts-maker/engine"
	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/subtitles"
)

func TestCreateSubtitles(t *testing.T) {
	transcriptionJson, err := os.Open("./segments.json")

	if err != nil {
		t.Logf("Error when opening segment")
	}

	defer transcriptionJson.Close()

	byteValue, err := io.ReadAll(transcriptionJson)

	var transcriptionOutput *models.TranscriptionOutput

	err = json.Unmarshal(byteValue, &transcriptionOutput)

	if err != nil {
		t.Logf("Error when opening segment")
	}

	subs := subtitles.CreateSubtitles(transcriptionOutput)

	out, err := subtitles.CreateSubtitleImages(subs)

	if err != nil {
		t.Logf("Error when opening segment")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	_, err = engine.AddSubtitlesToVideo(ctx, "../input.mp4", out, "out.mp4")

	if err != nil {
		fmt.Print(err)
	}

	fmt.Print(out)
}
