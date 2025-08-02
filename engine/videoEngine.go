package engine

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

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

	if len(clips) == 0 {
		return nil, fmt.Errorf("no clips supplied")
	}

	switch transition {
	case "none":
		return concatFastPath(clips, output)
	case "fade":
		return crossFadePath(clips, output)
	default:
		return nil, fmt.Errorf("unsupported transition %q", transition)
	}
}

// ---------- helpers ----------

func concatFastPath(clips []models.VideoWithTimestamp, output string) (*Video, error) {
	tmp, err := os.CreateTemp("", "concat_*.txt")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())

	for _, c := range clips {
		// concat demuxer expects:  file '/path'
		fmt.Fprintf(tmp, "file '%s'\n", filepath.ToSlash(c.Path))
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	cmd := exec.Command(
		"ffmpeg", "-y",
		"-f", "concat", "-safe", "0",
		"-i", tmp.Name(),
		"-c", "copy",
		output,
	)
	if b, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("concat copy: %w\n%s", err, b)
	}

	var total float64
	for _, c := range clips {
		total += c.Timestamp
	}
	return &Video{Path: output, Duration: total}, nil
}

func crossFadePath(clips []models.VideoWithTimestamp, output string) (*Video, error) {
	const fadeDur = 1.0 // seconds

	// ---------- build argv ----------
	var argv []string
	for _, c := range clips {
		argv = append(argv, "-i", c.Path)
	}

	// ---------- template-driven filter_complex ----------
	type seg struct {
		Idx       int
		Offset    float64
		FadeDur   float64
		LastIndex bool
	}

	// compute offsets (start times where each cross-fade begins)
	var segments []seg
	offset := 0.0
	for i, c := range clips[:len(clips)-1] {
		segments = append(segments, seg{
			Idx:       i,
			Offset:    offset,
			FadeDur:   fadeDur,
			LastIndex: false,
		})
		offset += c.Timestamp - fadeDur
	}
	segments = append(segments, seg{Idx: len(clips) - 1, LastIndex: true})

	// generate filter graph
	var graph bytes.Buffer
	tpl := template.Must(template.New("filt").Parse(`
{{- /* normalise streams */ -}}
{{- range $i, $c := .Clips }}
	[{{$i}}:v]scale=1080:-2,format=yuv420p[v{{$i}}];
	[{{$i}}:a]aformat=fltp:44100:stereo[a{{$i}}];
{{- end}}
{{- /* chain xfade + acrossfade */ -}}
{{- range $s := .Segments }}
	{{- if not $s.LastIndex }}
	[v{{$s.Idx}}][v{{$s.Idx | add 1}}]xfade=transition=fade:duration={{printf "%.2f" $s.FadeDur}}:offset={{printf "%.2f" $s.Offset}}[v{{$s.Idx | add 1}}x];
	[a{{$s.Idx}}][a{{$s.Idx | add 1}}]acrossfade=d={{printf "%.2f" $s.FadeDur}}[a{{$s.Idx | add 1}}x];
	{{- end }}
{{- end}}
`))

	// helper func for template math
	tpl.Funcs(template.FuncMap{
		"add": func(i int) int { return i + 1 },
	})

	if err := tpl.Execute(&graph, map[string]any{
		"Clips":    clips,
		"Segments": segments,
	}); err != nil {
		return nil, err
	}

	last := len(clips) - 1
	argv = append(argv,
		"-filter_complex", graph.String(),
		"-map", fmt.Sprintf("[v%dx]", last),
		"-map", fmt.Sprintf("[a%dx]", last),
		"-c:v", "libx264", "-preset", "medium", "-crf", "20",
		"-c:a", "aac", "-b:a", "192k",
		"-pix_fmt", "yuv420p",
		"-y", output,
	)

	cmd := exec.Command("ffmpeg", argv...)
	if b, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("xfade encode: %w\n%s", err, b)
	}

	var total float64
	for _, c := range clips {
		total += c.Timestamp
	}
	return &Video{Path: output, Duration: total}, nil
}
