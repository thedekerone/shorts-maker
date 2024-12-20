package subtitles

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/thedekerone/shorts-maker/engine"
	"github.com/thedekerone/shorts-maker/models"
)

type SubtitleStyles struct {
	FontFamily  string
	BorderColor string
	Color       string
	BorderWidth string
	FontSize    int
}

type Subtitle struct {
	Text      string
	Style     *SubtitleStyles
	StartTime float32
	EndTime   float32
}

func getDefaultStyles() SubtitleStyles {
	return SubtitleStyles{
		FontFamily:  "arial",
		BorderColor: "white",
		Color:       "white",
		BorderWidth: "1px",
		FontSize:    120,
	}
}

func CreateSubtitle(text string, style *SubtitleStyles, startTime float32, endTime float32) Subtitle {
	subtitle := Subtitle{
		Text:      text,
		Style:     style,
		StartTime: startTime,
		EndTime:   endTime,
	}

	return subtitle
}

func CreateSubtitles(transcript *models.TranscriptionOutput) []Subtitle {
	var subtitles []Subtitle
	defaultStyles := getDefaultStyles()

	for _, segment := range transcript.Segments {
		subtitles = append(subtitles, Subtitle{
			Text:      segment.Text,
			EndTime:   float32(segment.End),
			StartTime: float32(segment.Start),
			Style:     &defaultStyles,
		})
	}

	return subtitles
}

func CreateSubtitleImage(subs *Subtitle, subtitlesPath string) (engine.SubtitleImage, error) {
	var subImage engine.SubtitleImage

	subtitlesId := uuid.New().String()

	imageName := fmt.Sprintf("%s.png", subtitlesId)
	err := engine.RenderText(subs.Text, subtitlesPath, imageName)

	if err != nil {
		return subImage, errors.New("Failed to render text")
	}

	subImage.ImagePath = fmt.Sprintf("./%s/%s", subtitlesPath, imageName)
	subImage.StartTime = subs.StartTime
	subImage.EndTime = subs.EndTime

	return subImage, nil
}

func CreateSubtitleImages(subtitles []Subtitle) ([]engine.SubtitleImage, error) {
	var images []engine.SubtitleImage
	for _, subtitle := range subtitles {
		subtitleImages, err := CreateSubtitleImage(&subtitle, "example_subtitle")

		if err != nil {
			return nil, errors.New("Failed to create subtitle image")
		}

		images = append(images, subtitleImages)
	}

	return images, nil
}
