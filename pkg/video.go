package pkg

import (
	"fmt"
	"io"
	"net/http"
	"os"

	gobra "github.com/thedekerone/gobra/video"
)

func MakeVideoOfImages(imageUrls []string, duration int, outputFolder string) (string, error) {
	images := make([]string, len(imageUrls))
	for i, url := range imageUrls {
		fileName := fmt.Sprintf("%simage_%d.jpg", outputFolder, i)
		err := DownloadFile(url, fileName)
		if err != nil {
			return "", fmt.Errorf("failed to download image %s: %v", url, err)
		}
		images[i] = fileName
	}

	//delete all create images

	defer func() {
		for _, image := range images {
			os.Remove(image)
		}
	}()

	var video []*gobra.Video

	config := gobra.Config{
		Width:       1080,
		Height:      1920,
		Fps:         30,
		AspectRatio: 9.0 / 16.0,
	}

	durationByImage := duration / len(images) //duration of each image

	for _, image := range images {
		currentVideo := gobra.NewZoomPanVideoFromImage(image, durationByImage, 1.5, config)
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
