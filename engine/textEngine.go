package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type SubtitleImage struct {
	ImagePath string
	StartTime float32
	EndTime   float32
}

type TextStyle struct {
	Color       string
	FontSize    int
	Font        string
	Background  string
	BorderColor string
	BorderSize  int
}

type TextClip struct {
	Path      string
	StartTime float32
	EndTime   float32
	Duration  float32
}

func createDefaultStyle() *TextStyle {
	defaultStyle := TextStyle{
		Color:       "white",
		FontSize:    72,
		Font:        "Roboto-Black",
		Background:  "transparent",
		BorderColor: "black",
		BorderSize:  4,
	}

	return &defaultStyle
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
	defaultStyle := createDefaultStyle()

	if err != nil {
		return errors.New("error when creating folder")
	}

	cmd := exec.Command("magick",
		"-gravity", "center",
		"-stroke", defaultStyle.BorderColor,
		"-strokewidth", fmt.Sprintf("%d", defaultStyle.BorderSize),
		"-background", defaultStyle.Background,
		"-fill", defaultStyle.Color,
		"-font", defaultStyle.Font,
		"-pointsize", fmt.Sprintf("%d", defaultStyle.FontSize),
		"-size", "1080x1920",
		label,
		fullPath,
	)

	fmt.Printf("%s\n", cmd.String())

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func RenderTextWithStyles(text string, path string, filename string, styles *TextStyle) error {
	label := fmt.Sprintf("caption:%s", text)
	fullPath := fmt.Sprintf("./%s/%s", path, filename)

	err := os.MkdirAll("./"+path, os.ModePerm)

	if err != nil {
		return errors.New("error when creating folder")
	}

	cmd := exec.Command("magick",
		"-gravity", "center",
		"-stroke", styles.BorderColor,
		"-strokewidth", fmt.Sprintf("%d", styles.BorderSize),
		"-background", styles.Background,
		"-fill", styles.Color,
		"-font", styles.Font,
		"-pointsize", fmt.Sprintf("%d", styles.FontSize),
		"-size", "1080x1920",
		label,
		fullPath,
	)

	fmt.Printf("%s\n", cmd.String())

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func CreateTextClip(subImage SubtitleImage, output string) (*TextClip, error) {

	cmdArgs := append([]string{
		"-loop", "1",
		"-i", subImage.ImagePath,
		"-c:v", "libx264",
		"-t", fmt.Sprintf("%f", subImage.EndTime-subImage.StartTime),
		"-pix_fmt", "yuv420p",
		"-vf", "scale=320:240",
		output,
	})

	cmd := exec.Command("ffmpeg", cmdArgs...)

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	textClip := TextClip{
		Path:      output,
		StartTime: subImage.StartTime,
		EndTime:   subImage.EndTime,
		Duration:  subImage.EndTime - subImage.StartTime,
	}

	return &textClip, nil
}

func AddTextClipsToVideo(ctx context.Context, videoPath string, textClips []*TextClip, outputPath string) (string, error) {
	var textClipInputs []string
	var overlayFilters []string

	baseVideoString := "0"
	for i, textClip := range textClips {
		textClipInputs = append(textClipInputs, "-i", textClip.Path)
		overlayFilters = append(overlayFilters, fmt.Sprintf("[%s][%d] overlay=0:0:enable='between(t,%.2f,%.2f)'[out];", baseVideoString, i+1, textClip.StartTime, textClip.EndTime))
		baseVideoString = "out"
	}

	cmdArgs := append([]string{"-hwaccel", "auto", "-i", videoPath}, textClipInputs...)
	cmdArgs = append(cmdArgs,
		"-filter_complex",
		strings.Join(overlayFilters, ";"),
		"-pix_fmt", "yuv420p",
		"-c:a", "copy",
		"-preset", "faster",
		"-movflags", "+faststart",
		"-c:v", "libx264",
		"-crf", "23",
		outputPath, "-y")

	cmd := exec.CommandContext(ctx, "ffmpeg", cmdArgs...)

	print("command: ")
	print(cmd.String())
	print("\n")

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return "CREATED", nil
}

func convertImageToInVideo(imagePath string, startTime float32, endTime float32) (string, error) {
	outputPath := strings.TrimSuffix(imagePath, ".png") + "_in.mp4"
	duration := endTime - startTime

	cmd := exec.Command("ffmpeg",
		"-i", imagePath,
		"-vf", "scale='if(gt(iw,ih),-1,1080)':'if(gt(iw,ih),1920,-1)',zoompan=z='min(zoom+0.0015,1.5)':d=125,format=yuva420p",
		"-c:v", "libx264",
		"-t", fmt.Sprintf("%f", duration),
		"-pix_fmt", "yuva420p",
		outputPath,
	)

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return outputPath, nil
}

func AddSubtitlesToVideo(ctx context.Context, videoPath string, subImages []SubtitleImage, outputPath string) (string, error) {
	var imageAdditionFilter string
	var imageInputs []string
	baseVideoString := "0"

	for i, subImage := range subImages {
		//create scale in video from image

		imageVideoPath, err := convertImageToInVideo(subImage.ImagePath, subImage.StartTime, subImage.EndTime)

		if err != nil {
			return "", err
		}
		imageInputs = append(imageInputs, "-i", imageVideoPath)
		intermediate := fmt.Sprintf("out%d", i)
		imageAdditionFilter += fmt.Sprintf("[%s][%d]overlay=(main_w-overlay_w)/2:(main_h-overlay_h)-100:enable='between(t,%.2f,%.2f)'", baseVideoString, i+1, subImage.StartTime, subImage.EndTime)

		if i < len(subImages)-1 {
			imageAdditionFilter += fmt.Sprintf("[%s]", intermediate)
			imageAdditionFilter += ";"
		}

		baseVideoString = intermediate
	}

	cmdArgs := append([]string{"-hwaccel", "auto", "-i", videoPath}, imageInputs...)
	cmdArgs = append(cmdArgs,
		"-filter_complex",
		imageAdditionFilter,
		"-pix_fmt", "yuva420p",
		"-c:a", "copy",
		"-preset", "faster",
		"-movflags", "+faststart",
		"-c:v", "libx264",
		"-crf", "23",
		outputPath, "-y")

	cmd := exec.CommandContext(ctx, "ffmpeg", cmdArgs...)

	print("command: ")
	print(cmd.String())
	print("\n")

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return "CREATED", nil
}

func AddAssSubtitlesToVideo(ctx context.Context, videoPath string, subtitlesPath string, outputPath string) (string, error) {

	cmdArgs := append([]string{"-hwaccel", "auto", "-i", videoPath})
	cmdArgs = append(cmdArgs,
		"-vf", fmt.Sprintf("ass=%s", subtitlesPath),
		"-pix_fmt", "yuva420p",
		"-c:a", "copy",
		"-preset", "faster",
		"-movflags", "+faststart",
		"-c:v", "libx264",
		"-crf", "23",
		outputPath, "-y")

	cmd := exec.CommandContext(ctx, "ffmpeg", cmdArgs...)

	print("command: ")
	print(cmd.String())
	print("\n")

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return "CREATED", nil
}
