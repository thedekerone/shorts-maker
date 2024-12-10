package engine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/google/uuid"
	"github.com/thedekerone/shorts-maker/subtitles"
)

type SubtitleImage struct {
	imagePath string
	startTime int
	endTime   int
}

func folderExists(path string) (bool, error) {
	_, err := os.Stat(path)

	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

func RenderText(text string, path string, filename string) error {
	label := fmt.Sprintf("caption:%s", text)
	fullPath := fmt.Sprintf("./%s/%s", path, filename)

	err := os.MkdirAll("./"+path, os.ModePerm)

	if err != nil {
		return errors.New("error when creating folder")
	}

	cmd := exec.Command("convert",
		"-background", "lightblue",
		"-fill", "blue",
		"-font", "Candice",
		"-size", "1080x1920",
		"-gravity", "center",
		"-pointsize", "36",
		label,
		fullPath,
	)

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func CreateSubtitleImage(subs *subtitles.Subtitle, subtitlesPath string) (SubtitleImage, error) {
	var subImage SubtitleImage

	subtitlesId := uuid.New().String()

	imageName := fmt.Sprintf("%s.png", subtitlesId)
	err := RenderText(subs.Text, subtitlesPath, imageName)

	if err != nil {
		return subImage, errors.New("Failed to render text")
	}

	subImage.imagePath = subtitlesId + imageName
	subImage.startTime = subs.StartTime
	subImage.endTime = subs.EndTime

	return subImage, nil
}
