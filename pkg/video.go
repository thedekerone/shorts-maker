package pkg

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	gobra "github.com/thedekerone/gobra/video"
	"github.com/thedekerone/shorts-maker/models"
)

func MakeVideoOfImages(imagesWithTS []models.ImageWithTimestamp, duration float32, outputFolder string) (string, error) {
	images := make([]string, len(imagesWithTS))
	for i, image := range imagesWithTS {
		uniqueName := generateUniqueName()
		fileName := filepath.Join(outputFolder, fmt.Sprintf("%s.jpg", uniqueName))
		err := DownloadFile(image.URL, fileName)
		if err != nil {
			return "", fmt.Errorf("failed to download image %s: %v", image.URL, err)
		}
		images[i] = fileName
	}

	defer func() {
		// Delete all created images
		for _, img := range images {
			if err := os.Remove(img); err != nil {
				fmt.Printf("Failed to delete image %s: %v\n", img, err)
			}
		}
	}()

	var video []*gobra.Video
	config := gobra.Config{
		Width:       1080,
		Height:      1920,
		Fps:         30,
		AspectRatio: 9.0 / 16.0,
	}
	interval := float32(duration)/float32(len(images)) + 0.2
	for _, image := range images {
		currentVideo := gobra.NewZoomPanVideoFromImage(image, interval, 1.3, config)
		currentVideo = currentVideo.AddFadeIn(0.2)
		currentVideo = currentVideo.AddFadeOut(0.2)
		video = append(video, currentVideo)
	}
	merged := gobra.MergeVideos(video...)

	outputFile := filepath.Join(outputFolder, fmt.Sprintf("%s.mp4", generateUniqueName()))
	merged.Save(outputFile)
	return outputFile, nil
}

func MakeVideoOfLocalImages(imagesWithTS []models.ImageWithTimestamp, duration float32, outputFolder string) (string, error) {
	var video []*gobra.Video
	config := gobra.Config{
		Width:       1080,
		Height:      1920,
		Fps:         30,
		AspectRatio: 9.0 / 16.0,
	}
	interval := float32(duration)/float32(len(imagesWithTS)) + 0.2
	for _, image := range imagesWithTS {
		currentVideo := gobra.NewZoomPanVideoFromImage(image.URL, interval, 2.3, config)
		currentVideo = currentVideo.AddFadeIn(0.2)
		currentVideo = currentVideo.AddFadeOut(0.2)
		video = append(video, currentVideo)
	}

	merged := gobra.MergeVideos(video...)

	outputFile := filepath.Join(outputFolder, fmt.Sprintf("%s.mp4", generateUniqueName()))

	print(merged.AddSubtitles)
	merged.Save(outputFile)
	return outputFile, nil
}

func generateUniqueName() string {
	timestamp := time.Now().UnixNano()
	uuid := uuid.New().String()
	return fmt.Sprintf("%d_%s", timestamp, uuid)
}

func AddAudioToVideo(videoPath, audioPath, outputFolder string) (string, error) {
	// Generate unique names for temporary audio file and output video file
	outputFileName := fmt.Sprintf("%s.mp4", generateUniqueName())

	outputFilePath := filepath.Join(outputFolder, outputFileName)

	// Defer cleanup of temporary audio file
	defer func() {
		if err := os.Remove(audioPath); err != nil {
			fmt.Printf("Failed to remove temporary audio file: %v\n", err)
		}
	}()

	video := gobra.NewVideoWithAudio(videoPath, audioPath, gobra.Config{
		Width:       1080,
		Height:      1920,
		Fps:         30,
		AspectRatio: 9.0 / 16.0,
	})

	// Save video with subtitles
	if err := video.Save(outputFilePath); err != nil {
		return "", fmt.Errorf("failed to save video with subtitles: %v", err)
	}

	return outputFilePath, nil
}

func DownloadFile(url, fileName string) error {
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, response.Body)
	return err
}

func MergeAudios(audioUrls []string, outputFolder string) (string, error) {
	var audios []*gobra.Audio
	var tempFiles []string

	for _, url := range audioUrls {
		randomName := GenerateRandomString(8)
		fileName := fmt.Sprintf("%s%s.mp3", outputFolder, randomName)
		err := DownloadFile(url, fileName)
		if err != nil {
			return "", fmt.Errorf("failed to download audio %s: %v", url, err)
		}
		audio := gobra.NewAudio(fileName)
		audios = append(audios, audio)
		tempFiles = append(tempFiles, fileName)
	}

	outputPath := outputFolder + "merged_audio.mp3"
	err := gobra.MergeAudios(outputPath, audios...)

	for _, file := range tempFiles {
		os.Remove(file)
	}

	if err != nil {
		return "", err
	}

	return outputPath, nil
}

func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}
