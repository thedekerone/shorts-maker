package engine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

type SubtitleImage struct {
	ImagePath string
	StartTime float32
	EndTime   float32
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
		"-background", "transparent",
		"-fill", "blue",
		"-font", "Candice",
		"-gravity", "center",
		"-pointsize", "108",
		"-size", "1080x1920",
		label,
		fullPath,
	)

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func AddSubtitlesToVideo(videoPath string, subImages []SubtitleImage, outputPath string) (string, error) {
	var imageAdditionFilter string
	var imageInputs []string

	for i, subImage := range subImages {
		imageInputs = append(imageInputs, "-i", subImage.ImagePath)
		imageAdditionFilter += fmt.Sprintf("[%d:v][%d:v] overlay=(main_w-overlay_w)/2:(main_h-overlay_h)-100:enable='between(t,%.2f,%.2f)'", 0, i+1, subImage.StartTime, subImage.EndTime)
		if i < len(subImages)-1 {
			imageAdditionFilter += ";"
		}
	}

	cmdArgs := append([]string{"-i", videoPath}, imageInputs...)
	cmdArgs = append(cmdArgs, "-filter_complex", imageAdditionFilter, "-pix_fmt", "yuv420p", "-c:a", "copy", outputPath, "-y")

	cmd := exec.Command("ffmpeg", cmdArgs...)

	print("command: ")
	print(cmd.String())
	print("\n")
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return "CREATED", nil
}
