// Package engine renders short videos from a set of still images via FFmpeg.
// The implementation focuses on deterministic output, strict validation,
// graceful cancellation and detailed error reporting so it can safely run in
// a production workflow.
package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/thedekerone/shorts-maker/models"
)

const (
	// Supported transition effects.
	TransitionTypeFade  = "fade"
	TransitionTypeSlide = "slide"

	portraitAspect  = "1080x1920"
	landscapeAspect = "1920x1080"

	defaultFPS          = 30               // Frames per second
	defaultFadeDuration = time.Second      // 1 s fade
	defaultTimeout      = 45 * time.Second // ffmpeg execution timeout
)

// Video describes the rendered output file.
// Duration is the total play time (wall‑clock).
// Path is an absolute or relative filesystem path to the generated file.
type Video struct {
	Path     string
	Duration time.Duration
}

// Options exposes configurable parameters of the rendering pipeline.
// A nil *Options is interpreted as requesting defaults.
type Options struct {
	FPS          int           // Output frame‑rate (≥ 1)
	FadeDuration time.Duration // Duration of fade/xfade transitions
	Transition   string        // TransitionTypeFade (default) or TransitionTypeSlide
	Mode         string        // "portrait" (default) or "landscape"
	Rand         *rand.Rand    // RNG for subtle zoom offsets (deterministic if provided)
	Timeout      time.Duration // ffmpeg process timeout
	Logger       *log.Logger   // Optional logger (nil disables logging)
	FFmpegBinary string        // Path/name of ffmpeg executable (default "ffmpeg")
}

// Render stitches the provided images into a single video file.
//
//   - ctx controls cancellation and overall timeout.
//   - images must have at least one element; each timestamp defines its weight.
//   - dst is the target filename; ".mp4" is appended if missing.
//   - totalDuration specifies the wanted playtime.
//   - opts controls optional behaviour; pass nil for defaults.
//
// On success a *Video struct is returned.
func Render(ctx context.Context, images []models.ImageWithTimestamp, dst string, totalDuration time.Duration, opts *Options) (*Video, error) {
	// ---- defensive checks --------------------------------------------------
	if len(images) == 0 {
		return nil, errors.New("images must not be empty")
	}
	if totalDuration <= 0 {
		return nil, errors.New("totalDuration must be positive")
	}

	dst = filepath.Clean(dst)
	if filepath.Ext(dst) == "" {
		dst += ".mp4"
	}

	cfg := applyDefaults(opts)

	normaliseDurations(images, totalDuration)

	args, err := buildArguments(images, dst, cfg)
	if err != nil {
		return nil, err
	}

	// ---- invoke ffmpeg -----------------------------------------------------
	cctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, cfg.FFmpegBinary, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if cfg.Logger != nil {
		cfg.Logger.Printf("running: %s %s", cfg.FFmpegBinary, strings.Join(args, " "))
	}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w\n%s", err, stderr.String())
	}

	return &Video{Path: dst, Duration: totalDuration}, nil
}

// applyDefaults returns an Options instance with every field populated.
func applyDefaults(in *Options) *Options {
	if in == nil {
		in = &Options{}
	}
	if in.FPS == 0 {
		in.FPS = defaultFPS
	}
	if in.FadeDuration == 0 {
		in.FadeDuration = defaultFadeDuration
	}
	if in.Transition == "" {
		in.Transition = TransitionTypeFade
	}
	if in.Mode == "" {
		in.Mode = "portrait"
	}
	if in.Rand == nil {
		in.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	if in.Timeout == 0 {
		in.Timeout = defaultTimeout
	}
	if in.FFmpegBinary == "" {
		in.FFmpegBinary = "ffmpeg"
	}
	return in
}

// normaliseDurations mutates img.Timestamp values so that their sum equals the
// required total length. Relative weights are preserved.
func normaliseDurations(images []models.ImageWithTimestamp, total time.Duration) {
	var sum float64
	for _, img := range images {
		sum += img.Timestamp
	}
	factor := total.Seconds() / sum
	for i := range images {
		images[i].Timestamp *= factor
	}
}

// buildArguments assembles the full ffmpeg command‑line.
func buildArguments(images []models.ImageWithTimestamp, output string, cfg *Options) ([]string, error) {
	fade := cfg.FadeDuration.Seconds()
	fps := cfg.FPS
	rng := cfg.Rand

	var args []string

	// --- input files --------------------------------------------------------
	for _, img := range images {
		args = append(args, "-t", fmt.Sprintf("%.3f", img.Timestamp), "-i", img.URL)
	}

	// --- filter_complex -----------------------------------------------------
	aspect := portraitAspect
	if cfg.Mode == "landscape" {
		aspect = landscapeAspect
	}

	var filters []string
	var concats []string

	for idx, img := range images {
		zoom := fmt.Sprintf(
			"zoompan=z='if(lte(ot,%.2f),1.4,max(zoom-0.003,1.15))':d=%.2f:x='iw/2-(iw/zoom/2)+sin(ot*%.2f/3)*100':y='ih/2-(ih/zoom/2)-cos(ot*%.2f/2)*30':s=%s",
			img.Timestamp-rng.Float64()*img.Timestamp,
			img.Timestamp*float64(fps-5),
			rng.Float64()*2+1,
			rng.Float64()*2+1,
			aspect,
		)

		var b strings.Builder
		fmt.Fprintf(&b, "[%d:v]scale=4000:-1,setsar=1,%s", idx, zoom)

		switch cfg.Transition {
		case TransitionTypeSlide:
			if idx == 0 {
				fmt.Fprintf(&b, "[vin%d];", idx)
			} else {
				fmt.Fprintf(&b, "[vin%d];[vin%d][vin%d]xfade=transition=fade:duration=%.1f:offset=%.1f[v%d];", idx, idx-1, idx, fade, img.Timestamp-fade, idx)
			}
		case TransitionTypeFade:
			fallthrough
		default:
			if idx == 0 {
				fmt.Fprintf(&b, ",fade=t=out:st=%.2f:d=%.2f[v%d];", img.Timestamp-fade, fade, idx)
			} else {
				fmt.Fprintf(&b, ",fade=t=in:st=0:d=1,fade=t=out:st=%.2f:d=%.2f[v%d];", img.Timestamp-fade, fade/2, idx)
			}
		}

		filters = append(filters, b.String())
		concats = append(concats, fmt.Sprintf("[v%d]", idx))
	}

	concat := fmt.Sprintf("%s concat=n=%d:v=1:a=0,format=yuv420p[v]", strings.Join(concats, ""), len(concats))
	filterComplex := fmt.Sprintf("%s %s", strings.Join(filters, ""), concat)

	// --- final arguments ----------------------------------------------------
	args = append(args, "-filter_complex", filterComplex, "-map", "[v]", "-pix_fmt", "yuv420p", "-r", fmt.Sprint(fps), output, "-y")
	return args, nil
}
