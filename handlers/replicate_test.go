package handlers_test

import (
	"os"
	"testing"

	"github.com/thedekerone/shorts-maker/engine"
	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/pkg"
)

func TestImageVideo(t *testing.T) {
	// Test audio transcription

	imagesForVideo := []string{
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_3028651160.jpg",
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_3180086704.jpg",
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_3028651160.jpg",
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_3180086704.jpg",
	}

	var images []models.ImageWithTimestamp
	totalDuration := 40.0
	interval := totalDuration / float64(len(imagesForVideo))

	for _, v := range imagesForVideo {
		images = append(images, models.ImageWithTimestamp{
			URL:       v,
			Timestamp: interval,
		})
	}

	t.Log("Creating video from images...")

	path, err := engine.CreateVideoFromImages(images, os.TempDir()+pkg.GenerateRandomString(6)+".mp4", 20.0, engine.TransitionTypeFade, "")
	t.Log("Created video with images...")

	if err != nil {
		t.Fatalf("%v", err)
	}

	t.Log(path)
}
