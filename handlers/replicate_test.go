package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/thedekerone/shorts-maker/engine"
	"github.com/thedekerone/shorts-maker/models"
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
		"/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-0.jpeg",
		"/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-1.jpeg",
		"/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-2.jpeg",
		"/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-3.jpeg", "/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-1.jpeg",
		"/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-2.jpeg",
		"/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-3.jpeg", "/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-1.jpeg",
		"/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-2.jpeg",
		"/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-3.jpeg",
		"/Users/mauriciofow/Documents/shorts-maker/handlers/testImages/pexels-photo-4.jpeg",
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

func TestVideoGeneration(t *testing.T) {
	rs, err := services.NewReplicateService()

	if err != nil {
		t.Fatalf("Failed trying to connect to replicate service")
	}

	script := `It’s hard being an old man all alone. So I hired a live-in nurse.
It’s no fun being old.

Cloudy eyes. Brittle bones. Muscles gone soft. It’s not easy to get used to.

But at least I had Jeanine.

She was my home health aide. About 25. A pretty little thing. Being so vulnerable around a stranger was uncomfortable, at first. But her friendly demeanor soon put my mind at ease. As I showed her to her room, she kept going on about how nice the house was.

“Wow, Mr. Stephens! This place is gorgeous.”

“With what I paid for it, it had better be,” I joked, as she held my arm.

“Sir,” she said, smiling as she looked around, “I think we’re going to be good friends.”

And she meant it. Jeanine helped me with everything — chores, cooking, keeping track of my bills, always with a smile. Eventually, I could hardly remember how I ever got by without her. Her three month contract soon became six, at my request. Then nine. And each time, she seemed more than happy to stay. Insistent on it, as a matter of fact.

By New Year’s Eve, she’d been with me for nearly a year.

We’d just finished watching the ball drop. I was about to go to bed when I noticed Jeanine looked…different, as she asked me a question.

“Jim, where’s the money?”

“You want a raise?”, I chuckled.

“Don’t bullshit me!” she hissed, her words dripping with frustration.

“I only took this job because I heard you were loaded. And I’m getting what I came for.”

As it dawned on me that she was serious, I noticed the gun she’d pulled from her jacket pocket.

“An old man in a big house, all alone. Cash. Jewelry.” She gestured towards the stairs with her gun. “Take me to them.”

Begrudgingly, I led her to the safe hidden in my bedroom closet. She forced me to open it, but not before I spit in her eye. Liar. Without blinking, she put a bullet in my chest and began to rummage through my valuables.

Just what I’d been waiting for.

I don’t know what she realized first, that there was no money, or that she couldn’t move. She collapsed, paralyzed by my venom as the ragged hole in my ribs closed before her eyes. As I slid my proboscis down her throat, she gazed up at me in agony as her face began to sag and wrinkle. And for a split second, just before the transformation was complete, she didn’t see Jim Stephens’ face. She didn’t see her own.

She saw mine.

Taking the shape of the rich old man had been fun, for a while. But as I changed into Jeanine’s clothes, I was ready for something fresh. Soon, police would find “Jim Stevens” dead on his bedroom floor. No one would ask questions. And I’d have a new body, one dripping with opportunity.

As I practiced sobbing with Jeanine’s voice before dialing 911, I smiled.

What is it human kids say?

“New Year, New Me.”`

	// Test audio generation

	voice, err := rs.GetVoice(script)

	if err != nil {
		t.Fatalf("Failed to generate voice")
	}

	t.Logf("%s", voice)

	// Test audio transcription

	transcription, err := rs.GetTranscription(voice, script)
	if err != nil {
		t.Fatalf("Failed to transcribe audio")
	}

	imagesForVideo := []string{
		"./testImages/pexels-photo-0.jpeg",
		"./testImages/pexels-photo-1.jpeg",
		"./testImages/pexels-photo-2.jpeg",
		"./testImages/pexels-photo-3.jpeg",
		"./testImages/pexels-photo-4.jpeg",
	}

	lastSegment := transcription.Segments[len(transcription.Segments)-1]

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

	path, err := pkg.MakeVideoOfLocalImages(images, float32(lastSegment.End), os.TempDir())
	t.Log("Created video with images...")

	if err != nil {
		t.Fatalf("Failed to Create video of images")
	}

	t.Log("Starting to create video with sound...")
	outputPath, err := pkg.AddAudioToVideo(path, voice, os.TempDir())

	if err != nil {
		t.Fatalf("Failed to Create video with sound")
	}

	outputFileName := fmt.Sprintf("%s.mp4", pkg.GenerateRandomString(6))
	outputFilePath := filepath.Join(os.TempDir(), outputFileName)

	animationSubs := subtitles.CreateSubtitles(transcription)

	subtitleImages, err := subtitles.CreateSubtitleImages(animationSubs)

	if err != nil {
		t.Fatalf("Failed to Create video with subtitles")

	}

	ctx := context.Background()
	str, err := engine.AddSubtitlesToVideo(ctx, outputPath, subtitleImages, outputFilePath)

	if err != nil {
		t.Fatalf("Failed to Create video with subs")

	}

	t.Logf("successfully generated %s", str)

}
