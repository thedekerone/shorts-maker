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

	script := `I Didn't Mean to Destroy The Most Precious Thing In The World To Me
	I leaned my head back against the wall in the Emergency room at our local hospital, tears pouring down my face. And it wasn’t just from the pain either. The swelling and rash had already gone down after several anti-histamine and other anti-allergen shots, but my heart was breaking for my poor darling Cara.

	I didn’t mean to. I can’t believe I killed the most precious thing to me in the world. 

	I looked at myself in the selfie view of my phone camera. I still looked deadly ill. The police had questioned me, but there was nothing more than a squashed bloody spider in my bedroom, and they had to let me go. They said they would search for Cara as they helped the paramedics get me out of there. I heard them talking about “mental breakdown” and “paranoid” with the emerg intake. 

	That day had been like most other days I spent with Cara. We were in bed, and it was her “turn”. I slipped my fingers over her dazzling silken skin, feeling her soft and loveable under my hands. 

	And then, I tried to repress the familiar shudder as her limbs elongated and she sprouted four more, bristles poked out of her smooth skin, her head grew large and her eyes multiplied. I rolled away from her.

	A spider as big as a beach ball stood quivering on the bed where Cara had been buckling and crying out in pleasure a second ago. The transformation was very fast.

	And it only lasted a few minutes, mercifully. I tried to control my face and body so she couldn’t see my fear, which had never lessened, not one iota, through all these months.

	I hated and feared spiders since childhood, but that had never come up in the very early days of our relationship. 

	About three weeks into what had been the best relationship of my life so far, Cara decided she trusted me and told me the reason why she hadn’t let me make her orgasm.

	“I turn into a spider” she had murmured.

	I froze. I knew immediately she wasn’t joking or mad, simply telling the bald truth.

	“No-one else knows. I’ve never orgasmed with a partner before.” She snuggled up to me. “There was a mirror next to my bed when I was a child. I was, you know, experimenting, and then it happened. I could see the spider in the mirror.”

	I couldn’t say anything. She looked up at me, worry shadowing her beautiful green eyes. “You don’t mind do you? It doesn’t change anything- I- I love you so much- I’ve never told anyone - I want to be with you properly, let you do all the things to me-” she pressed against me, naked, and my heart had melted even as I became aroused. I drew her close and whispered “shhh, baby it’s ok. I would love you even if you turned into a worm, remember?”

	She laugh-cried and then opened up to me. I reached deep inside her, and soon enough, she orgasmed.

	That had been six months ago. I always let go of her as soon as she started transforming, so I wouldn’t have to feel her body shrinking and ballooning, the limbs growing and the bristles. Oh the bristles.

	I couldn’t get used to it. I walked to the bedroom window. It was getting worse. Because now Cara’s love had grown, she wanted me to hold her while she came, to pet her while she was in spider form. She wanted more. She never said so, but I knew, by the look of reproach and longing on her beautiful face as she flickered back into human form. And she had been talking about marriage and commitment. 

	She was only a spider for a few minutes. And everything else was perfect.

	A movement caught my eye- I turned. She was scuttling towards me. She had never done that before. Wordlessly understanding my aversion, she had always respected my distance while she was a spider.

	But now she was approaching. I took a step back, impulsively reached down, grabbed my slipper and raised it.

	The large spider jumped on me and then bit, releasing venom into my blood. I screamed in agony and then I lashed out with the slipper. The pain and horror befuddling me, the slipper squashed my beloved Cara fully. I fell howling to the floor in a paroxysm of grief and pain. 

	I will never love again.`

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

	path, err := pkg.MakeVideoOfLocalImages(images, float32(lastSegment.End), os.TempDir()+pkg.GenerateRandomString(6)+".mp4")

	if err != nil {
		t.Fatalf("Failed to Create video of images")
	}
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
