package subtitles

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

func CreateShortLengthSubtitles(transcript *models.TranscriptionOutput) []Subtitle {
	var subtitles []Subtitle
	defaultStyles := getDefaultStyles()

	for _, segment := range transcript.Segments {

		for _, w := range segment.Words {

			subtitles = append(subtitles, Subtitle{
				Text:      w.Word,
				EndTime:   float32(w.End),
				StartTime: float32(w.Start),
				Style:     &defaultStyles,
			})
		}

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

func CreateShortSubsWithStyles(transcript *models.TranscriptionOutput, styles *SubtitleStyles) []Subtitle {
	var subtitles []Subtitle
	maxChars := 6

	var prevEnd float32
	for _, segment := range transcript.Segments {
		var combined []models.Word
		lenSum := 0

		fmt.Printf("%v", segment.Words)

		for i, w := range segment.Words {
			combined = append(combined, w)
			lenSum = len(w.Word) + lenSum

			if lenSum >= maxChars || i == len(segment.Words)-1 {
				sentence := ""

				for _, w := range combined {
					sentence = sentence + " " + strings.ToUpper(w.Word)
				}

				start := float32(combined[0].Start)

				if float32(combined[0].Start) == 0 && i != 0 {
					start = prevEnd + 0.1
				}

				subtitles = append(subtitles, Subtitle{
					Text:      sentence,
					EndTime:   float32(combined[len(combined)-1].End),
					StartTime: start,
					Style:     styles,
				})

				prevEnd = float32(combined[len(combined)-1].End)

				combined = make([]models.Word, 0)
				lenSum = 0
			}
		}

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

// Dialogue: 0,0:00:01.00,0:00:03.00,Default,,0,0,0,,{\fscx50\fscy50\t(0,60,\fscx55\fscy55)\t(60,140,\fscx50\fscy50)}Hello, world!
func CreateAssFile(subtitles []Subtitle, basePath string) (string, error) {
	// Ensure the basePath is an absolute path
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	println("GOT TO ASS")

	text, err := getTextFromFile(absBasePath)
	println("TEXT TO ASS")

	if err != nil {
		return "", err
	}

	println("TEXT TOsfdad sdas adsdsa ads adsds ASS")

	for _, v := range subtitles {
		formattedString := fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0,0,0,,{\\fscx40\\fscy40\\t(0,60,\\fscx45\\fscy45)\\t(60,140,\\fscx40\\fscy40)}%s\n", transformFloatToTimestamp(v.StartTime), transformFloatToTimestamp(v.EndTime), v.Text)

		text = text + formattedString
	}

	println("subs---------------------")

	tempFile, err := os.CreateTemp("", "*.ass")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	_, err = tempFile.WriteString(text)
	if err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

func transformFloatToTimestamp(time float32) string {
	hours := int(time) / 3600
	minutes := (int(time) % 3600) / 60
	seconds := int(time) % 60
	milliseconds := int((time - float32(int(time))) * 100)

	return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, seconds, milliseconds)
}

func getTextFromFile(path string) (string, error) {
	text := ""
	f, err := os.Open(path)

	if err != nil {
		return "", err
	}

	r := bufio.NewReader(f)

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}

		fmt.Print(line)
		text = text + line + "\n"
	}

	defer f.Close()

	return text, nil

}
