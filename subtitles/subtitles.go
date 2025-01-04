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
	BorderWidth int
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
		FontFamily:  "Roboto-Black",
		BorderColor: "white",
		Color:       "white",
		BorderWidth: 1,
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

func CreateSubtitlesWithStyles(transcript *models.TranscriptionOutput, styles *SubtitleStyles) []Subtitle {
	var subtitles []Subtitle

	for _, segment := range transcript.Segments {
		subtitles = append(subtitles, Subtitle{
			Text:      segment.Text,
			EndTime:   float32(segment.End),
			StartTime: float32(segment.Start),
			Style:     styles,
		})
	}

	return subtitles
}

func CreateSubtitleImage(subs *Subtitle, subtitlesPath string) (engine.SubtitleImage, error) {
	var subImage engine.SubtitleImage

	subtitlesId := uuid.New().ID()

	imageName := fmt.Sprintf("%d.png", subtitlesId)
	err := engine.RenderTextWithStyles(subs.Text, subtitlesPath, imageName, getTextStyles(subs.Style))

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

func getTextStyles(s *SubtitleStyles) *engine.TextStyle {
	engineStyles := engine.TextStyle{
		Color:       s.Color,
		FontSize:    s.FontSize,
		Font:        s.FontFamily,
		Background:  "transparent",
		BorderColor: s.BorderColor,
		BorderSize:  s.BorderWidth,
	}
	return &engineStyles
}
