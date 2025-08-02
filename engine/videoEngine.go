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

const (
	TransitionTypeFade  = "fade"
	TransitionTypeSlide = "slide"
)

func createTransitionFilter(imageIndex int, image models.ImageWithTimestamp, totalDuration float64, fadeDuration float64, fps int, transitionType string, mode string) string {
	aspect := "1080x1920"

	if mode == "landscape" {
		aspect = "1920x1080"
	}

	filter := fmt.Sprintf("[%d:v]scale=4000:-1,setsar=1,", imageIndex)
	zoomFilter := fmt.Sprintf("zoompan=z='if(lte(ot,%.2f),1.4, max(zoom-0.003,1.15))':d=%.2f:x='iw/2-(iw/zoom/2)+sin(ot*%.2f/3)*100':y='ih/2-(ih/zoom/2)-cos(ot*%.2f/2)*30':s=%s", image.Timestamp-rand.Float64()*(image.Timestamp), image.Timestamp*float64(fps-5), rand.Float64()*2+1, rand.Float64()*2+1, aspect)

	switch transitionType {
	case TransitionTypeFade:
		if imageIndex == 0 {
			filter += fmt.Sprintf("%s,fade=t=out:st=%.1f:d=%.1f[v%d];", zoomFilter, image.Timestamp-fadeDuration, fadeDuration, imageIndex)
		} else {
			filter += fmt.Sprintf("%s,fade=t=in:st=0:d=1,fade=t=out:st=%.1f:d=%.1f[v%d];", zoomFilter, image.Timestamp-fadeDuration, fadeDuration/2, imageIndex)
		}

	case TransitionTypeSlide:
		if imageIndex == 0 {
			filter += fmt.Sprintf("%s[vin%d];", zoomFilter, imageIndex)
		} else {
			filter += fmt.Sprintf("%s[vin%d];[vin%d][vin%d]xfade=transition=fade:duration=%.1f:offset=%.1f[v%d];", zoomFilter, imageIndex, imageIndex-1, imageIndex, fadeDuration, image.Timestamp-fadeDuration, imageIndex)

		}

	default:
		// Default to fade transition if no valid type is provided
		if imageIndex == 0 {
			filter += fmt.Sprintf("%s,fade=t=out:st=%.1f:d=%.1f[v%d];", zoomFilter, image.Timestamp-fadeDuration, fadeDuration, imageIndex)
		} else {
			filter += fmt.Sprintf("%s,fade=t=in:st=0:d=1,fade=t=out:st=%.1f:d=%.1f[v%d];", zoomFilter, image.Timestamp-fadeDuration, fadeDuration/2, imageIndex)
		}
	}

	return filter
}

func createConcatenationFilter(images []models.ImageWithTimestamp) string {
	var concats []string
	for i := range images {
		concats = append(concats, fmt.Sprintf("[v%d]", i))
	}
	return fmt.Sprintf("%s concat=n=%d:v=1:a=0,format=yuv420p[v]", strings.Join(concats, ""), len(concats))
}
func createConcatenationFilterForClips(clips []models.VideoWithTimestamp) string {
	var concats []string
	for i := range clips {
		concats = append(concats, fmt.Sprintf("[%d]", i))
	}
	return fmt.Sprintf("%s concat=n=%d:v=1:a=0,format=yuv420p[v]", strings.Join(concats, ""), len(concats))
}

func CreateVideoFromImages(images []models.ImageWithTimestamp, output string, totalDuration float64, transitionType string, mode string) (*Video, error) {
	var imagePaths []string
	fps := 30
	fadeDuration := 1.0

	for _, v := range images {
		imagePaths = append(imagePaths, "-t", fmt.Sprintf("%.2f", v.Timestamp), "-i", v.URL)
	}

	var filterComplexes []string
	accumulatedDuration := 0.0

	for i, v := range images {
		if i == len(images)-1 {
			v.Timestamp = totalDuration - accumulatedDuration
		}
		accumulatedDuration += v.Timestamp

		filterComplex := createTransitionFilter(i, v, totalDuration, fadeDuration, fps, transitionType, mode)
		filterComplexes = append(filterComplexes, filterComplex)
	}

	concatenationFilter := createConcatenationFilter(images)

	framerate := fmt.Sprintf("%d", fps)
	cmdArgs := append([]string{"-framerate", framerate}, imagePaths...)
	cmdArgs = append(cmdArgs,
		"-filter_complex", fmt.Sprintf("%s %s", strings.Join(filterComplexes, ""), concatenationFilter),
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

func CreateVideoFromClips(
	clips []models.VideoWithTimestamp,
	output string,
	transition string, // "none" | "fade"
	totalDuration float64,
) (*Video, error) {
	var clipsPaths []string

	for _, v := range clips {
		clipsPaths = append(clipsPaths, "-i", v.Path)
	}

	var filterComplexes []string

	concatenationFilter := createConcatenationFilterForClips(clips)

	cmdArgs := append(clipsPaths,
		"-filter_complex", fmt.Sprintf("%s %s", strings.Join(filterComplexes, ""), concatenationFilter),
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
