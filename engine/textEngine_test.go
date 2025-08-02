package engine_test

import (
	"testing"

	"github.com/thedekerone/shorts-maker/engine"
	"github.com/thedekerone/shorts-maker/models"
)

func _estRenderText(t *testing.T) {
	text := "this is an example of a test with subtitles la ala all ala l"

	err := engine.RenderText(text, "testsImages", "image_test.png")

	if err != nil {
		t.Log(err)
		t.Fatalf("Failed to run render text")
	}

	t.Log("good")
}

func TestVideoGen(t *testing.T) {
	clip1 := models.VideoWithTimestamp{
		Path:      "~/Documents/personal/cortos-dev-env/shorts-maker/assets/generate_image_0.mp4",
		Timestamp: 5.0,
	}
	clip2 := models.VideoWithTimestamp{
		Path:      "~/Documents/personal/cortos-dev-env/shorts-maker/assets/generate_image_1.mp4",
		Timestamp: 5.0,
	}

	clips := []models.VideoWithTimestamp{clip1, clip2}
	video, err := engine.CreateVideoFromClips(clips, "xd.mp4", "fade", 10.0)

	if err != nil {
		println(err.Error())
	}

	println(video)
}
