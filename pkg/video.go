package pkg

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"

	gobra "github.com/thedekerone/gobra/video"
	"github.com/thedekerone/shorts-maker/models"
)

func MakeVideoOfImages(imagesWithTS []models.ImageWithTimestamp, duration float32, outputFolder string) (string, error) {
	images := make([]string, len(imagesWithTS))
	for i, image := range imagesWithTS {
		fileName := fmt.Sprintf("%simage_%d.jpg", outputFolder, i)
		err := DownloadFile(image.URL, fileName)
		if err != nil {
			return "", fmt.Errorf("failed to download image %s: %v", image.URL, err)
		}
		images[i] = fileName
	}

	//delete all create images

	var video []*gobra.Video

	config := gobra.Config{
		Width:       1080,
		Height:      1920,
		Fps:         30,
		AspectRatio: 9.0 / 16.0,
	}

	interval := float32(duration) / 4

	for _, image := range images {
		currentVideo := gobra.NewZoomPanVideoFromImage(image, interval, 1.5, config)
		currentVideo = currentVideo.AddFadeIn(1)
		currentVideo = currentVideo.AddFadeOut(1)
		video = append(video, currentVideo)

	}

	merged := gobra.MergeVideos(video...)

	merged.Save(outputFolder + "output.mp4")

	return outputFolder + "output.mp4", nil
}

func AddAudioToVideo(videoPath, audioPath, outputFolder string) (string, error) {
	//download audio file
	err := DownloadFile(audioPath, outputFolder+"audio.mp3")
	if err != nil {
		return "", fmt.Errorf("failed to download audio file: %v", err)
	}

	video := gobra.NewVideoWithAudio(videoPath, outputFolder+"audio.mp3", gobra.Config{
		Width:       1080,
		Height:      1920,
		Fps:         30,
		AspectRatio: 9.0 / 16.0,
	})

	//create fake subtitles file

	subtitles := outputFolder + "testing.ass"

	video.SaveWithSubtitles(outputFolder+"output2.mp4", subtitles)

	return outputFolder + "output2.mp4", nil
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
		randomName := generateRandomString(8)
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

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}
