package engine

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/thedekerone/shorts-maker/models"
)

type Video struct {
	Path     string
	Duration float64
}

func CreateVideoFromImages(images []models.ImageWithTimestamp, output string) (*Video, error) {
	var imagePaths []string
	var totalDuration float64

	if len(images) > 1 {
		totalDuration = images[len(images)-1].Timestamp - images[0].Timestamp
	}

	for _, v := range images {
		imagePaths = append(imagePaths, "-loop", "1", "-t", fmt.Sprintf("%.2f", v.Timestamp), "-i", v.URL)
	}

	var filterComplexes []string
	for i, v := range images {
		filter := ""
		if i == 0 {
			filter = fmt.Sprintf("[0:v]scale=1280:720:force_original_aspect_ratio=decrease,pad=1280:720:(ow-iw)/2:(oh-ih)/2,setsar=1,fade=t=out:st=%.1f:d=1[v0];", v.Timestamp-1.0)
		} else {
			filter = fmt.Sprintf("[%d:v]scale=1280:720:force_original_aspect_ratio=decrease,pad=1280:720:(ow-iw)/2:(oh-ih)/2,setsar=1,fade=t=in:st=0:d=1,fade=t=out:st=%.1f:d=1[v%d];", i, v.Timestamp-1.0, i)
		}

		filterComplexes = append(filterComplexes, filter)
	}

	var concats []string

	for i, _ := range images {
		concats = append(concats, fmt.Sprintf("[v%d]", i))
	}

	framerate := "30"

	cmdArgs := append([]string{"-framerate", framerate}, imagePaths...)
	cmdArgs = append(cmdArgs,
		"-filter_complex", fmt.Sprintf("%s %s", strings.Join(filterComplexes, ""), strings.Join(concats, "")+fmt.Sprintf("concat=n=%d:v=1:a=0,format=yuv420p[v]", len(concats))),
		"-map", "[v]",
		"-pix_fmt", "yuv420p",
		output, "-y",
	)

	cmd := exec.Command("ffmpeg", cmdArgs...)
	println(cmd.String())
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("error creating video: %w", err)
	}

	video := Video{
		Path:     output,
		Duration: totalDuration,
	}

	return &video, nil
}
func CreateVideoFromImage(Image models.ImageWithTimestamp, duration float64) {

}
