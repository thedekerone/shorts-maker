package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedekerone/shorts-maker/engine"
	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/neets"
	"github.com/thedekerone/shorts-maker/pkg"
	"github.com/thedekerone/shorts-maker/services"
	"github.com/thedekerone/shorts-maker/subtitles"
)

func skipTestCreateVideoFromImages(t *testing.T) {
	transcriptionJson, err := os.Open("../subtitles/segments.json")
	voice := "https://replicate.delivery/yhqm/KxTMgiSffThXK0efD4aeGXSvvO1ltXTv3zqZyql8em5fszafTA/output.wav"
	if err != nil {
		t.Logf("Error when opening segment")
	}

	defer transcriptionJson.Close()

	byteValue, err := io.ReadAll(transcriptionJson)

	var transcription *models.TranscriptionOutput

	err = json.Unmarshal(byteValue, &transcription)

	if err != nil {
		t.Logf("Error when opening segment")
	}

	t.Log("dsadsaadsadsdsa\n")

	imagesForVideo := []string{
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_575969719.jpg",
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_1020201603.jpg",
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_3694320876.jpg",
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_4280124063.jpg",
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_1460845745.jpg",
	}
	t.Log("dsadsaadsadsdsa2\n")

	var images []models.ImageWithTimestamp
	totalDuration := transcription.Segments[len(transcription.Segments)-1].End
	interval := totalDuration / float64(len(imagesForVideo))

	for _, v := range imagesForVideo {
		images = append(images, models.ImageWithTimestamp{
			URL:       v,
			Timestamp: interval,
		})
	}

	t.Log("dsadsaadsadsdsa3\n")

	path, err := engine.CreateVideoFromImages(images, os.TempDir()+pkg.GenerateRandomString(6)+".mp4")

	if err != nil {
		t.Fatalf("Failed to Create video of images")
	}
	outputPath, err := pkg.AddAudioToVideo(path.Path, voice, os.TempDir())

	t.Logf("%s", outputPath)
}

func skipTestVideoGeneration(t *testing.T) {
	rs, err := services.NewReplicateService()

	if err != nil {
		t.Fatalf("Failed trying to connect to replicate service")
	}

	script := `I Gave My Husband an Ultimatum Today
When my husband got home from work, I was waiting for him in the bedroom.

“Since you chose to stay with me,” I smiled, “I’m going to need your help disposing of this.”`

	// Test audio generation

	n := neets.CreateNeets()

	vr := n.NewVoiceRequest(strings.ReplaceAll(script, "\n", ""), "grimes")

	audioPath, err := vr.Call("test.mp3")
	if err != nil {
		t.Fatalf("%v", err)
	}

	if err != nil {
		t.Fatalf("Failed to generate voice")
	}

	t.Logf("%s", audioPath)

	// Test audio transcription
	transcription, err := rs.GetTranscription(audioPath, script)
	if err != nil {
		t.Fatalf("Failed to transcribe audio")
	}

	imagesForVideo := []string{
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_472479805.jpg",
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_575969719.jpg",
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_1020201603.jpg",
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_3694320876.jpg",
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_4280124063.jpg",
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_1460845745.jpg",
	}

	var images []models.ImageWithTimestamp
	totalDuration := transcription.Segments[len(transcription.Segments)-1].End
	interval := totalDuration / float64(len(imagesForVideo))

	for _, v := range imagesForVideo {
		images = append(images, models.ImageWithTimestamp{
			URL:       v,
			Timestamp: interval,
		})
	}

	t.Log("Creating video from images...")

	path, err := engine.CreateVideoFromImages(images, os.TempDir()+pkg.GenerateRandomString(6)+".mp4")
	t.Log("Created video with images...")

	if err != nil {
		t.Fatalf("Failed to Create video of images")
	}

	t.Log("Starting to create video with sound...")
	outputPath, err := pkg.AddAudioToVideo(path.Path, audioPath, os.TempDir())

	if err != nil {
		t.Fatalf("Failed to Create video with sound")
	}

	outputFileName := fmt.Sprintf("%s.mp4", pkg.GenerateRandomString(6))
	outputFilePath := filepath.Join(os.TempDir(), outputFileName)

	subStyles := subtitles.SubtitleStyles{
		FontFamily:  "Roboto-Black",
		FontSize:    72,
		BorderColor: "red",
		BorderWidth: 4,
		Color:       "white",
	}
	animationSubs := subtitles.CreateShortSubsWithStyles(transcription, &subStyles)

	t.Logf("======================================================")
	baseAssPath := "../handlers/assets/base.ass"
	subtitlesPath, err := subtitles.CreateAssFile(animationSubs, baseAssPath)

	t.Logf(subtitlesPath)

	if err != nil {
		t.Logf("%v", err)

		t.Fatalf("Failed to Create video with subtitles")

	}

	ctx := context.Background()
	str, err := engine.AddAssSubtitlesToVideo(ctx, outputPath, subtitlesPath, outputFilePath)

	if err != nil {
		t.Logf("%v", err)
		t.Fatalf("Failed to Create video with subs")
	}

	t.Logf("successfully generated %s", str)

}

func TestImageVideo(t *testing.T) {
	// Test audio transcription

	imagesForVideo := []string{
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_3028651160.jpg",
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_3180086704.jpg",
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_3028651160.jpg",
		"/var/folders/27/3tlwn24s2d7bgwgclkt3y2hw0000gn/T/image_3180086704.jpg",
	}

	var images []models.ImageWithTimestamp
	totalDuration := 40.0
	interval := totalDuration / float64(len(imagesForVideo))

	for _, v := range imagesForVideo {
		images = append(images, models.ImageWithTimestamp{
			URL:       v,
			Timestamp: interval,
		})
	}

	t.Log("Creating video from images...")

	path, err := engine.CreateVideoFromImages(images, os.TempDir()+pkg.GenerateRandomString(6)+".mp4")
	t.Log("Created video with images...")

	if err != nil {
		t.Fatalf("%v", err)
	}

	t.Log(path)
}
