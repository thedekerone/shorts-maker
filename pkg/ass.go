package pkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/thedekerone/shorts-maker/models"
)

func CreateDialog(segment models.Segment) (string, error) {
	baseAssDialog := fmt.Sprintf("\nDialogue: 0,%s,%s,Ashita,,0000,0000,0000,karaoke,%s", floatToAssTimeStamp(segment.Start), floatToAssTimeStamp(segment.End), GetWordsFromSentence(segment))

	return baseAssDialog, nil
}

func GetWordsFromSentence(sentence models.Segment) string {

	words := ""

	for _, word := range sentence.Words {
		/**
		if i != len(sentence.Words)-1 {
			words = words + getWordTiming(word, sentence.Words[i+1].Start-word.End) + " "
		} else if i == 0 {
			words = words + getWordTiming(word, word.Start-sentence.Start) + " "
		} else {
			words = words + getWordTiming(word, 0) + " "
		}
		**/

		words = words + getWordTiming(word, 0) + " "
	}

	return words

}

func getWordTiming(word models.Word, offset float64) string {
	time := int((word.End - word.Start + offset) * 100)

	return fmt.Sprintf("{\\k%d}%s", time, word.Word)
}

func floatToAssTimeStamp(time float64) string {
	// 145.345

	absoluteInteger := int(time) // returns 145

	seconds := float64(absoluteInteger%60) + (time - float64(absoluteInteger))

	absoluteInteger = absoluteInteger - absoluteInteger%60

	minutes := absoluteInteger / 60

	absoluteInteger = absoluteInteger - minutes*60

	hours := absoluteInteger / 3600

	return fmt.Sprintf("%d:%02d:%05.2f", hours, minutes, seconds)

}

func CreateAssFile(fileName string, transcription models.TranscriptionOutput) error {
	baseAssFile, err := os.ReadFile(filepath.Join("assets", "base.ass"))

	if err != nil {
		return errors.New("Failed to open base ASS file")
	}

	assFile, err := os.Create(fileName)

	defer assFile.Close()

	if err != nil {
		return err
	}

	segments := transcription.Segments

	_, err = assFile.Write(baseAssFile)

	if err != nil {
		return errors.New("failed to write base file into new file")
	}

	for _, segment := range segments {
		dialog, err := CreateDialog(segment)

		if err != nil {
			return errors.New("failed to create dialog format")
		}
		_, err = assFile.Write([]byte(dialog))

		if err != nil {
			return errors.New("failed to write dialog format")
		}
	}

	return nil

}
