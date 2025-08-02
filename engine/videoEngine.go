package engine

import (
	"fmt"
	"math/rand/v2"
	"os"
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
) (*Video, error) {

	// Fast path: no transitions → use concat demuxer (no re-encode)
	if transition == "none" {
		tmp, _ := os.CreateTemp("", "concat_*.txt")
		for _, c := range clips {
			// ffmpeg concat-demuxer needs paths one per line:   file '/path'
			fmt.Fprintf(tmp, "file '%s'\n", c.Path)
		}
		tmp.Close()

		cmd := exec.Command(
			"ffmpeg", "-y",
			"-f", "concat", "-safe", "0",
			"-i", tmp.Name(),
			"-c", "copy",
			output,
		)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("concat copy: %w", err)
		}
	} else { // ---------- fade / xfade pipeline ----------
		/*
		   Build something like:

		   ffmpeg -y \
		     -i clip0.mp4 -i clip1.mp4 -i clip2.mp4 \
		     -filter_complex "
		       [0:v]scale=1080:-2,format=yuv420p[v0];
		       [1:v]scale=1080:-2,format=yuv420p[v1];
		       [2:v]scale=1080:-2,format=yuv420p[v2];
		       [0:a]aformat=fltp:44100:stereo[a0];
		       [1:a]aformat=fltp:44100:stereo[a1];
		       [2:a]aformat=fltp:44100:stereo[a2];
		       [v0][a0][v1][a1]xfade=transition=fade:duration=1:offset=<t0>[x1][a1x];
		       [x1][a1x][v2][a2]xfade=transition=fade:duration=1:offset=<t1>[v][a]
		     " -map "[v]" -map "[a]" -c:v libx264 -c:a aac output.mp4
		*/

		var args []string
		for _, c := range clips {
			args = append(args, "-i", c.Path)
		}

		var filtParts []string
		for i := range clips {
			// normalise to 1080 width; change to 720 if you prefer
			vs := fmt.Sprintf("[%d:v]scale=1080:-2,format=yuv420p[v%d];", i, i)
			as := fmt.Sprintf("[%d:a]aformat=fltp:44100:stereo[a%d];", i, i)
			filtParts = append(filtParts, vs, as)
		}

		// now chain xfade
		// offset = cumulative duration minus 1 s fade
		offset := 0.0
		for i := 0; i < len(clips)-1; i++ {
			d := clips[i].Length
			xfade := fmt.Sprintf(
				"[v%d][a%d][v%d][a%d]xfade=transition=fade:duration=1:offset=%.2f[v%d][a%d];",
				i, i, i+1, i+1, offset, i+1, i+1)
			filtParts = append(filtParts, xfade)
			offset += d - 1
		}

		// final labels: v{last}, a{last}
		last := len(clips) - 1
		filt := strings.Join(filtParts, "")

		args = append(args,
			"-filter_complex", filt,
			"-map", fmt.Sprintf("[v%d]", last),
			"-map", fmt.Sprintf("[a%d]", last),
			"-c:v", "libx264", "-c:a", "aac",
			"-pix_fmt", "yuv420p",
			output, "-y",
		)

		if err := exec.Command("ffmpeg", args...).Run(); err != nil {
			return nil, fmt.Errorf("xfade encode: %w", err)
		}
	}

	total := 0.0
	for _, c := range clips {
		total += c.Length
	}
	return &Video{Path: output, Duration: total}, nil
}
