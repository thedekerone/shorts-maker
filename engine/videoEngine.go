package engine

import (
	"fmt"
	"math/rand/v2"
	"os/exec"
	"strings"

	"github.com/thedekerone/shorts-maker/models"
)

type Video struct {
	Path     string
	Duration float64
}

func CreateVideoFromImages(images []models.ImageWithTimestamp, output string, totalDuration float64) (*Video, error) {
	var imagePaths []string
	fps := 30

	for _, v := range images {
		imagePaths = append(imagePaths, "-t", fmt.Sprintf("%.2f", v.Timestamp), "-i", v.URL)
	}

	fadeDuration := 1.0

	accumulatedDuration := 0.0

	var filterComplexes []string
	for i, v := range images {

		if i == len(images)-1 {
			v.Timestamp = totalDuration - accumulatedDuration
		}

		accumulatedDuration = accumulatedDuration + v.Timestamp

		filter := fmt.Sprintf("[%d:v]scale=4000:-1,setsar=1,", i)
		zoomFilter := fmt.Sprintf("zoompan=z='if(lte(ot,%.2f),1.4, max(zoom-0.003,1.15))':d=%.2f:x='iw/2-(iw/zoom/2)+sin(ot*%.2f/3)*100':y='ih/2-(ih/zoom/2)-cos(ot*%.2f/2)*30':s=1080x1920", v.Timestamp-rand.Float64()*(v.Timestamp), v.Timestamp*float64(fps-5), rand.Float64()*2+1, rand.Float64()*2+1)

		if i == 0 {
			filter += fmt.Sprintf("%s,fade=t=out:st=%.1f:d=%.1f[v%d];", zoomFilter, v.Timestamp-fadeDuration, fadeDuration, i)
		} else {
			filter += fmt.Sprintf("%s,fade=t=in:st=0:d=1,fade=t=out:st=%.1f:d=%.1f[v%d];", zoomFilter, v.Timestamp-fadeDuration, fadeDuration/2, i)
		}

		filterComplexes = append(filterComplexes, filter)
	}

	var concats []string
	for i, _ := range images {
		concats = append(concats, fmt.Sprintf("[v%d]", i))
	}

	framerate := fmt.Sprintf("%d", fps)
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
