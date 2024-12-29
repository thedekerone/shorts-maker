package engine_test

import (
	"testing"

	"github.com/thedekerone/shorts-maker/engine"
	"github.com/thedekerone/shorts-maker/models"
)

func TestVideoFromImages(t *testing.T) {
	imagesForVideo := []string{
		"/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-0.jpeg",
		"/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-1.jpeg",
		"/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-2.jpeg",
		"/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-3.jpeg",
		"/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-4.jpeg",
	}
	t.Log("dsadsaadsadsdsa2\n")

	var images []models.ImageWithTimestamp

	for _, v := range imagesForVideo {
		images = append(images, models.ImageWithTimestamp{
			URL:       v,
			Timestamp: 3,
		})
	}

	video, err := engine.CreateVideoFromImages(images, "videoOutput.mp4")

	if err != nil {
		print("dasdas dasdas ads ads ads  dadas ")
		t.Fatalf("error creating vidoe from images")
	}

	t.Logf(video.Path)
}
