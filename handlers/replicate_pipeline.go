package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/thedekerone/shorts-maker/app/images"
	"github.com/thedekerone/shorts-maker/elevenlabs"
	"github.com/thedekerone/shorts-maker/engine"
	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/pkg"
	"github.com/thedekerone/shorts-maker/services"
	"github.com/thedekerone/shorts-maker/subtitles"
)

const klingMaxRetries = 3

func uploadGeneratedFile(mio *services.MinioService, filePath string, fileName string, jobID string) error {
	file, err := os.Open(filePath)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error opening file: "+err.Error())
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting file info: "+err.Error())
		return err
	}
	fileSize := fileInfo.Size()
	fileExt := strings.ToLower(filepath.Ext(fileInfo.Name()))
	generatedFileName := fmt.Sprintf("shorts/%s/%s%s", jobID, fileName, fileExt)

	contentType := mime.TypeByExtension(fileExt)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = mio.Client.PutObject(context.Background(), "shorts-maker", generatedFileName, file, fileSize, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error uploading file: "+err.Error())
		return err
	}

	return nil
}

func processVideoGeneration(jobID string, script string, webhook string, voiceID string, mode string, visualStyle string, captionStyle string, captionPosition string, opts generationOptions) error {
	job := newGenerationJob(jobID, normalizeMode(mode), webhook)
	if !opts.GenerateKling && !opts.GenerateImages {
		return job.fail("invalid_options", errors.New("no generation variant selected"))
	}
	defer job.cleanupFiles()

	log.Printf("job %s: starting generation", jobID)

	if err := job.initServices(); err != nil {
		return job.fail("init_services", err)
	}

	voicePath, transcript, err := generateVoice(jobID, script, voiceID)
	if err != nil {
		return job.fail("generate_voice", err)
	}
	job.addCleanup(voicePath)

	if transcript == nil {
		transcript, err = generateTranscription(jobID, job.replicate, voicePath, script)
		if err != nil {
			return job.fail("generate_transcription", err)
		}
	}

	shotPlan := elevenlabs.BuildShotPlan(transcript, 0.7, 4.0, 7.0)
	log.Printf("job %s: built %d shots", jobID, len(shotPlan.Shots))

	if err := uploadGeneratedFile(job.minio, voicePath, "voice_script", jobID); err != nil {
		return job.fail("upload_voice", err)
	}

	plan, err := generateImages(jobID, &shotPlan, job.mode, visualStyle)
	if err != nil {
		return job.fail("generate_images", err)
	}
	stills := plan.Shots
	captionCfg := adjustCaptionForMode(resolveCaptionStyle(captionStyle), job.mode)
	resolvedPosition := resolveCaptionPosition(job.mode, captionPosition)
	if len(stills) == 0 {
		return job.fail("generate_images", errors.New("no stills generated"))
	}
	for _, still := range stills {
		job.addCleanup(still.URL)
	}

	for i, still := range stills {
		if err := uploadGeneratedFile(job.minio, still.URL, fmt.Sprintf("generate_image_%d", i), jobID); err != nil {
			return job.fail("upload_image", err)
		}
	}

	var klingURL, imageURL string

	if opts.GenerateKling {
		klingClips, err := generateKlingVideos(jobID, job.replicate, stills, job.mode)
		if err != nil {
			return job.fail("generate_kling", err)
		}
		for _, clip := range klingClips {
			job.addCleanup(clip.Path)
		}

		klingVoiceCopy, err := duplicateFile(voicePath)
		if err != nil {
			return job.fail("copy_voice_kling", err)
		}
		job.addCleanup(klingVoiceCopy)

		klingVideoPath, err := createVideoFromClips(jobID, transcript, klingClips, klingVoiceCopy, job.mode, captionCfg, resolvedPosition)
		if err != nil {
			return job.fail("create_kling_video", err)
		}
		job.addCleanup(klingVideoPath)

		klingURL, err = uploadToMinio(jobID, "kling_video", job.minio, klingVideoPath)
		if err != nil {
			return job.fail("upload_kling_video", err)
		}
	}

	if opts.GenerateImages {
		imageVoiceCopy, err := duplicateFile(voicePath)
		if err != nil {
			return job.fail("copy_voice_images", err)
		}
		job.addCleanup(imageVoiceCopy)

		imageVideoPath, err := createVideo(jobID, transcript, stills, imageVoiceCopy, job.mode, captionCfg, resolvedPosition)
		if err != nil {
			return job.fail("create_image_video", err)
		}
		job.addCleanup(imageVideoPath)

		imageURL, err = uploadToMinio(jobID, "image_video", job.minio, imageVideoPath)
		if err != nil {
			return job.fail("upload_image_video", err)
		}
	}

	finalURL := ""
	if klingURL != "" {
		finalURL = klingURL
	}
	if imageURL != "" {
		finalURL = imageURL
	}

	if finalURL == "" {
		return job.fail("missing_outputs", errors.New("no video outputs generated"))
	}

	updateJobStatus(jobID, "completed", finalURL, "")

	if err := notifyWebhook(job.webhook, CompletedWebhookBody{ID: jobID, KlingURL: klingURL, ImagesURL: imageURL}); err != nil {
		log.Printf("job %s: webhook notify failed: %v", jobID, err)
	}

	log.Printf("job %s: generation completed", jobID)
	return nil
}

func connectToMinio(jobID string) (*services.MinioService, error) {
	updateJobStatus(jobID, "connecting_to_minio", "", "")
	minioClient, err := services.ConnectToMinio()
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Couldn't connect to minio: "+err.Error())
		return nil, err
	}
	return minioClient, nil
}

func createReplicateService(jobID string) (*services.ReplicateService, error) {
	updateJobStatus(jobID, "creating_replicate_service", "", "")
	rs, err := services.NewReplicateService()
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error creating replicate service: "+err.Error())
		return nil, err
	}
	return rs, nil
}

func generateVoice(jobID string, predictions string, voiceid string) (string, *models.TranscriptionOutput, error) {
	updateJobStatus(jobID, "generating_voice", "", "")
	n := elevenlabs.CreateEleven()
	if voiceid == "" || voiceid == "default" {
		voiceid = "NNl6r8mD7vthiJatiJt1"
	}

	vr := n.NewVoiceRequestMultilingual(predictions, voiceid)

	audio, err := vr.Call(os.TempDir()+pkg.GenerateRandomString(6)+".mp3", true)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting audio: "+err.Error())
		return "", nil, err
	}

	return audio.AudioPath, audio.Transcription, nil
}

func generateTranscription(jobID string, rs *services.ReplicateService, voice string, predictions string) (*models.TranscriptionOutput, error) {
	updateJobStatus(jobID, "generating_transcription", "", "")
	transcript, err := rs.GetTranscription(voice, predictions)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting transcription: "+err.Error())
		return nil, err
	}
	return transcript, nil
}

func generateImages(jobID string, shotPlan *elevenlabs.ShotPlan, mode string, visualStyle string) (*models.ImagePlan, error) {
	updateJobStatus(jobID, "generating_images", "", "")
	plan, err := images.GetImagesWithTimestamps(shotPlan, mode, visualStyle)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting images: "+err.Error())
		return nil, err
	}
	return plan, nil
}

func generateKlingVideos(jobID string, rs *services.ReplicateService, stills []models.ImageWithTimestamp, mode string) ([]models.VideoWithTimestamp, error) {
	if len(stills) == 0 {
		return nil, fmt.Errorf("no stills provided for Kling generation")
	}

	segments := buildKlingSegments(stills)
	if len(segments) == 0 {
		return nil, fmt.Errorf("failed to build Kling segments")
	}

	log.Printf("job %s: generating %d Kling segments from %d stills", jobID, len(segments), len(stills))

	updateJobStatus(jobID, "generating_kling_videos", "", "")
	qualityMode := klingQualityMode(mode)

	const maxConcurrentKlingJobs = 2

	clipped := make([]models.VideoWithTimestamp, len(segments))
	var (
		wg       sync.WaitGroup
		sem      = make(chan struct{}, maxConcurrentKlingJobs)
		once     sync.Once
		firstErr error
	)

	for i, seg := range segments {
		i := i
		seg := seg
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			prompt := strings.TrimSpace(seg.Prompt)
			if prompt == "" {
				prompt = "Cinematic shot, photorealistic"
			}

			variants := buildKlingPromptVariants(prompt)
			var (
				videoPath string
				lastErr   error
			)

			for attempt := 0; attempt < klingMaxRetries; attempt++ {
				candidate := variants[minInt(attempt, len(variants)-1)]
				videoPath, lastErr = rs.GenerateKlingVideo(candidate, seg.StartImage.URL, seg.Duration, qualityMode)
				if lastErr == nil {
					break
				}
				log.Printf("job %s: kling segment %d attempt %d failed: %v", jobID, i, attempt+1, lastErr)
			}

			if lastErr != nil {
				once.Do(func() { firstErr = fmt.Errorf("Error generating Kling video %d: %w", i, lastErr) })
				return
			}

			clipped[i] = models.VideoWithTimestamp{
				Path:      videoPath,
				Timestamp: float64(seg.Duration),
			}
		}()
	}

	wg.Wait()
	if firstErr != nil {
		updateJobStatus(jobID, "failed", "", firstErr.Error())
		return nil, firstErr
	}

	return clipped, nil
}

type klingSegment struct {
	StartImage models.ImageWithTimestamp
	Prompt     string
	Duration   int
}

func buildKlingSegments(stills []models.ImageWithTimestamp) []klingSegment {
	const (
		minDuration        = 5.0
		maxDuration        = 10.0
		preferTenThreshold = 8.0
	)

	segments := make([]klingSegment, 0)
	if len(stills) == 0 {
		return segments
	}

	chooseDuration := func(total float64) int {
		if total <= minDuration {
			return 5
		}
		return 10
	}

	var (
		currentStart   models.ImageWithTimestamp
		accumulatedDur float64
		prompts        []string
		startSet       bool
	)

	flush := func(duration int) {
		prompt := strings.Join(filterEmpty(prompts), "\n")
		if prompt == "" {
			prompt = "Cinematic shot, photorealistic"
		}
		segments = append(segments, klingSegment{
			StartImage: currentStart,
			Prompt:     prompt,
			Duration:   duration,
		})
		prompts = prompts[:0]
		accumulatedDur = 0
		startSet = false
	}

	for idx, still := range stills {
		if !startSet {
			currentStart = still
			startSet = true
		}
		if trimmed := strings.TrimSpace(still.Prompt); trimmed != "" {
			prompts = append(prompts, trimmed)
		}
		accumulatedDur += still.Timestamp

		nextDur := 0.0
		if idx+1 < len(stills) {
			nextDur = stills[idx+1].Timestamp
		}

		duration := 0
		switch {
		case accumulatedDur >= preferTenThreshold:
			duration = 10
		case accumulatedDur >= minDuration && (accumulatedDur+nextDur > maxDuration):
			duration = chooseDuration(accumulatedDur)
		case idx == len(stills)-1:
			duration = chooseDuration(accumulatedDur)
		}

		if duration != 0 && accumulatedDur > 0 {
			flush(duration)
		}
	}

	if startSet && accumulatedDur > 0 {
		flush(chooseDuration(accumulatedDur))
	}

	return segments
}

func filterEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func klingQualityMode(mode string) string {
	if strings.EqualFold(mode, "landscape") {
		return "pro"
	}
	return "standard"
}

func resolveCaptionPosition(mode string, requested string) string {
	pos := strings.TrimSpace(strings.ToLower(requested))
	switch pos {
	case "top", "bottom", "center":
		return pos
	}
	if strings.EqualFold(mode, "portrait") {
		return "center"
	}
	return "bottom"
}

func alignmentTagForPosition(pos string) string {
	switch pos {
	case "top":
		return "{\\an8}"
	case "center":
		return "{\\an5}"
	default:
		return "{\\an2}"
	}
}

const klingSafeSuffix = " PG-rated, family-friendly visuals; no nudity, no gore, no explicit content."

func buildKlingPromptVariants(base string) []string {
	sanitized := strings.TrimSpace(base)
	if sanitized == "" {
		sanitized = "Cinematic shot, photorealistic"
	}
	fallback := "Wide cinematic establishing shot of the scene, dynamic camera move, dramatic lighting."
	return []string{
		sanitized,
		sanitized + klingSafeSuffix,
		fallback + klingSafeSuffix,
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func createVideoFromClips(jobID string, transcript *models.TranscriptionOutput, clips []models.VideoWithTimestamp, voice string, mode string, captionCfg captionStyleConfig, position string) (string, error) {
	ctx := context.Background()

	updateJobStatus(jobID, "merging_clips", "", "")
	mergedPath := filepath.Join(os.TempDir(), pkg.GenerateRandomString(6)+".mp4")

	totalDuration := 0.0
	for _, clip := range clips {
		totalDuration += clip.Timestamp
	}

	vid, err := engine.CreateVideoFromClips(
		clips,
		mergedPath,
		engine.TransitionTypeFade,
		totalDuration,
	)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "merge clips: "+err.Error())
		return "", err
	}

	updateJobStatus(jobID, "adding_audio_to_video", "", "")
	outputPath, err := pkg.AddAudioToVideo(vid.Path, voice, os.TempDir())
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error adding audio to video: "+err.Error())
		return "", err
	}

	outputFileName := fmt.Sprintf("%s.mp4", generateUniqueName())
	outputFilePath := filepath.Join(os.TempDir(), outputFileName)
	subStyles := captionCfg.SubtitleStyles
	animationSubs := subtitles.CreateShortSubsWithStyles(transcript, &subStyles, captionCfg.Karaoke)

	updateJobStatus(jobID, "generating ASS file", "", "")
	baseAssPath, err := filepath.Abs(captionCfg.TemplatePath)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting absolute path for ASS template: "+err.Error())
		return "", err
	}
	fullPrefix := captionCfg.TextPrefix + alignmentTagForPosition(position)
	subtitlesPath, err := subtitles.CreateAssFile(animationSubs, baseAssPath, fullPrefix, captionCfg.Karaoke)
	if err != nil {
		return "", err
	}
	updateJobStatus(jobID, "Adding subtitles to video", "", "")

	if _, err = engine.AddAssSubtitlesToVideo(ctx, outputPath, subtitlesPath, outputFilePath); err != nil {
		return "", err
	}

	return outputFilePath, nil
}

func createVideo(jobID string, transcript *models.TranscriptionOutput, imagesWithTS []models.ImageWithTimestamp, voice string, mode string, captionCfg captionStyleConfig, position string) (string, error) {
	ctx := context.Background()
	updateJobStatus(jobID, "creating_subtitle_file", "", "")

	updateJobStatus(jobID, "creating_video_from_images", "", "")
	totalDuration := transcript.Segments[len(transcript.Segments)-1].End

	path, err := engine.CreateVideoFromImages(imagesWithTS, os.TempDir()+pkg.GenerateRandomString(6)+".mp4", totalDuration, engine.TransitionTypeFade, mode)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error making video: "+err.Error())
		return "", err
	}

	updateJobStatus(jobID, "adding_audio_to_video", "", "")
	outputPath, err := pkg.AddAudioToVideo(path.Path, voice, os.TempDir())
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error adding audio to video: "+err.Error())
		return "", err
	}

	outputFileName := fmt.Sprintf("%s.mp4", generateUniqueName())
	outputFilePath := filepath.Join(os.TempDir(), outputFileName)
	subStyles := captionCfg.SubtitleStyles
	animationSubs := subtitles.CreateShortSubsWithStyles(transcript, &subStyles, captionCfg.Karaoke)

	updateJobStatus(jobID, "generating ASS file", "", "")
	baseAssPath, err := filepath.Abs(captionCfg.TemplatePath)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting absolute path for base.ass: "+err.Error())
		return "", err
	}
	fullPrefix := captionCfg.TextPrefix + alignmentTagForPosition(position)
	subtitlesPath, err := subtitles.CreateAssFile(animationSubs, baseAssPath, fullPrefix, captionCfg.Karaoke)
	if err != nil {
		return "", err
	}
	updateJobStatus(jobID, "Adding subtitles to video", "", "")

	if _, err = engine.AddAssSubtitlesToVideo(ctx, outputPath, subtitlesPath, outputFilePath); err != nil {
		return "", err
	}

	return outputFilePath, nil
}

func uploadToMinio(jobID, variant string, minioClient *services.MinioService, outputFilePath string) (string, error) {
	updateJobStatus(jobID, "preparing_file_for_upload", "", "")
	file, err := os.Open(outputFilePath)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error opening file: "+err.Error())
		return "", err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting file info: "+err.Error())
		return "", err
	}
	fileSize := fileInfo.Size()
	fileExt := filepath.Ext(fileInfo.Name())
	generatedFileName := fmt.Sprintf("shorts/%s/%s%s", jobID, variant, fileExt)

	updateJobStatus(jobID, "uploading_to_minio", "", "")
	_, err = minioClient.Client.PutObject(context.Background(), "shorts-maker", generatedFileName, file, fileSize, minio.PutObjectOptions{ContentType: "video/mp4"})
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error uploading file to Minio: "+err.Error())
		return "", err
	}

	updateJobStatus(jobID, "generating_presigned_url", "", "")
	object, err := minioClient.Client.PresignedGetObject(context.Background(), "shorts-maker", generatedFileName, time.Hour*12, nil)
	if err != nil {
		updateJobStatus(jobID, "failed", "", "Error getting presigned url: "+err.Error())
		return "", err
	}

	videoSignedURL := object.String()
	setJobVideoURL(jobID, variant, videoSignedURL)

	return videoSignedURL, nil
}

func duplicateFile(src string) (string, error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()

	dst, err := os.CreateTemp("", "voice_copy_*.mp3")
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(dst, srcFile); err != nil {
		dst.Close()
		os.Remove(dst.Name())
		return "", err
	}

	if err := dst.Close(); err != nil {
		return "", err
	}

	return dst.Name(), nil
}

func notifyWebhook(target string, body CompletedWebhookBody) error {
	if target == "" {
		return nil
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("webhook responded %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	return nil
}
