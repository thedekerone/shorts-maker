package subtitles

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

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
	Tokens    []SubtitleToken
}

type SubtitleToken struct {
	Text     string
	Duration float32
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

func CreateShortSubsWithStyles(transcript *models.TranscriptionOutput, styles *SubtitleStyles, karaoke bool) []Subtitle {
	if transcript == nil {
		return nil
	}

	if styles == nil {
		defaultStyles := getDefaultStyles()
		styles = &defaultStyles
	}

	const (
		maxCaptionChars = 32
		maxCaptionWords = 8
		minGapSeconds   = 0.05
		minDuration     = 0.4
		maxDuration     = 4.0
	)

	var (
		subtitles []Subtitle
		chunk     []models.Word
		lastEnd   float32
	)

	flushChunk := func(force bool) {
		if len(chunk) == 0 {
			return
		}

		text := buildCaptionText(chunk)
		if text == "" {
			chunk = chunk[:0]
			return
		}

		start := float32(chunk[0].Start)
		if start < lastEnd+minGapSeconds {
			start = lastEnd + minGapSeconds
		}

		end := float32(chunk[len(chunk)-1].End)
		if end <= start+minDuration {
			end = start + minDuration
		}

		tokens := make([]SubtitleToken, 0, len(chunk))
		if karaoke {
			for _, w := range chunk {
				word := strings.TrimSpace(w.Word)
				if word == "" {
					continue
				}
				dur := float32(w.End - w.Start)
				if dur <= 0 {
					dur = minDuration
				}
				tokens = append(tokens, SubtitleToken{Text: word, Duration: dur})
			}
		}

		subtitles = append(subtitles, Subtitle{
			Text:      text,
			StartTime: start,
			EndTime:   end,
			Style:     styles,
			Tokens:    tokens,
		})

		lastEnd = end
		chunk = chunk[:0]
	}

	shouldBreak := func(chunk []models.Word, next models.Word) bool {
		if len(chunk) == 0 {
			return false
		}

		duration := float32(chunk[len(chunk)-1].End - chunk[0].Start)
		if duration >= maxDuration {
			return true
		}

		gap := next.Start - chunk[len(chunk)-1].End
		return gap >= 1.2
	}

	addWord := func(w models.Word) {
		nextChunk := append(chunk, w)
		chunk = nextChunk

		trimmed := strings.TrimSpace(w.Word)
		duration := float32(chunk[len(chunk)-1].End - chunk[0].Start)
		if captionLength(chunk) >= maxCaptionChars || len(chunk) >= maxCaptionWords || duration >= maxDuration || wordEndsSentence(trimmed) {
			flushChunk(false)
		}
	}

	for _, segment := range transcript.Segments {
		if len(segment.Words) == 0 {
			text := strings.TrimSpace(segment.Text)
			if text == "" {
				continue
			}
			chunk = chunk[:0]
			chunk = append(chunk, models.Word{Word: text, Start: segment.Start, End: segment.End})
			flushChunk(true)
			continue
		}

		for idx, w := range segment.Words {
			addWord(w)
			if idx < len(segment.Words)-1 && shouldBreak(chunk, segment.Words[idx+1]) {
				flushChunk(false)
			}
		}

		flushChunk(true)
	}

	flushChunk(true)

	return subtitles
}

func buildCaptionText(words []models.Word) string {
	var builder strings.Builder
	for _, w := range words {
		trimmed := strings.TrimSpace(w.Word)
		if trimmed == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte(' ')
		}
		builder.WriteString(strings.ToUpper(trimmed))
	}
	return builder.String()
}

func captionLength(words []models.Word) int {
	length := 0
	for i, w := range words {
		word := strings.TrimSpace(w.Word)
		if word == "" {
			continue
		}
		length += utf8.RuneCountInString(word)
		if i < len(words)-1 {
			length++
		}
	}
	return length
}

func wordEndsSentence(word string) bool {
	trimmed := strings.TrimSpace(word)
	if trimmed == "" {
		return false
	}
	switch trimmed[len(trimmed)-1] {
	case '.', '!', '?':
		return true
	default:
		return false
	}
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
func CreateAssFile(subtitles []Subtitle, basePath string, textPrefix string, karaoke bool) (string, error) {
	// Ensure the basePath is an absolute path
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	println("GOT TO ASS")

	text, err := getTextFromFile(absBasePath)
	println("TEXT TO ASS")

	if err != nil {
		println("error in assdsads ads ds dsa")
		println(err.Error())
		return "", err
	}

	println("TEXT TOsfdad sdas adsdsa ads adsds ASS")

	for _, v := range subtitles {
		text += formatDialogueLine(v, textPrefix, karaoke)
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

func formatDialogueLine(sub Subtitle, textPrefix string, karaoke bool) string {
	start := transformFloatToTimestamp(sub.StartTime)
	end := transformFloatToTimestamp(sub.EndTime)

	var builder strings.Builder
	builder.WriteString("Dialogue: 0,")
	builder.WriteString(start)
	builder.WriteByte(',')
	builder.WriteString(end)
	builder.WriteString(",Default,,0,0,0,,")
	builder.WriteString(textPrefix)

	if karaoke && len(sub.Tokens) > 0 {
		builder.WriteString(buildKaraokeText(sub.Tokens))
	} else {
		builder.WriteString(escapeASSText(sub.Text))
	}

	builder.WriteByte('\n')
	return builder.String()
}

func buildKaraokeText(tokens []SubtitleToken) string {
	var builder strings.Builder
	for i, token := range tokens {
		if token.Text == "" {
			continue
		}
		dur := math.Max(1, float64(token.Duration)*100)
		builder.WriteString(fmt.Sprintf("{\\k%d}%s", int(dur), escapeASSText(strings.ToUpper(token.Text))))
		if i < len(tokens)-1 {
			builder.WriteByte(' ')
		}
	}
	return builder.String()
}

func escapeASSText(text string) string {
	text = strings.ReplaceAll(text, "{", "(")
	text = strings.ReplaceAll(text, "}", ")")
	return text
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
