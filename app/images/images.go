package images

import (
	"fmt"

	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/services"
)

func GetImagesWithTimestamps(transcript *models.TranscriptionOutput, mode string) ([]models.ImageWithTimestamp, error) {
	rs, err := services.NewReplicateService()
	deepseek, err := services.NewDeepSeekService()
	if err != nil {
		return nil, fmt.Errorf("error creating replicate service: %w", err)
	}

	totalDuration := transcript.Segments[len(transcript.Segments)-1].End

	var imagesWithTimestamps []models.ImageWithTimestamp

	var segmentStrings string

	for _, v := range transcript.Segments {
		segmentStrings = segmentStrings + fmt.Sprintf("{ segment: %s, start: %.3f, end: %.3f } \n", v.Text, v.Start, v.End)
	}

	imageGenerationPrompts := fmt.Sprintf(
		`
		### SYSTEM ###
You are an **Image‑Prompt Composer**.

Your job is to turn a timestamped story into a sequence of ultra‑realistic, cinematic image prompts—returned as a single JSON object and nothing else.

INSTRUCTIONS
1. Read the story supplied between the triple quotes: 
   """
   %s
   """
2. **Identify key moments** (scene changes, emotional peaks, environment shifts).
3. Decide the number of images:  
   • ≥ 1 image every ~5 s.  
   • Keep pacing engaging, not frantic.  
4. Allocate each image’s on‑screen **duration** so that the sum equals %.2f (±0.01 s).
5. For every image craft a **stand‑alone prompt** that fully describes:  
   • Setting, subjects, action, mood.  
   • Lighting style (e.g., golden‑hour rim light).  
   • Camera details (lens, depth‑of‑field, framing, shot type).  
   • Stylistic tags: “8 K, photorealistic, cinematic color grade”.  
   (Assume the generator has no other context.)
6. Maintain a *single, coherent visual style* across all images—hyper‑real textures, lifelike lighting.
7. Output **only** the JSON below (no code fences, no comments).

OUTPUT FORMAT
{
  "numImages": <integer>,
  "images": [
    {
      "prompt": "<full scene description>",
      "duration": <float>   // seconds
    }
    // … additional images …
  ]
}

EXAMPLE
{
  "numImages": 3,
  "images": [
    {
      "prompt": "Wide‑angle sunrise shot of an isolated desert road stretching toward crimson mountains, warm golden‑hour light casting long shadows, crisp 50 mm lens, shallow depth of field, hyper‑realistic 8 K, cinematic color grade",
      "duration": 11.5
    },
    {
      "prompt": "Macro close‑up of a weathered hand gripping a rusty compass, soft ambient backlight revealing skin texture, f/2.8, filmic grain, photorealistic 8 K",
      "duration": 12.0
    },
    {
      "prompt": "Lone traveler silhouetted beneath a vast starlit sky on a windswept plateau, cool moonlight, slow dolly‑out 35 mm, HDR, ultra‑real 8 K",
      "duration": 13.0
    }
  ]
}

`, fmt.Sprintf("\n %v", segmentStrings), totalDuration)

	promptForImage, err := deepseek.
		GetCompletitionForImages(imageGenerationPrompts, "")

	println("%v", promptForImage)
	for i := 0; i < int(promptForImage.NumImages); i++ {
		images, err := rs.GetImages(promptForImage.ImagesPrompt[i].Prompt, 1, mode)

		if err != nil {
			return nil, fmt.Errorf("error getting image %d: %w", i+1, err)
		}

		if len(images) > 0 {
			imagesWithTimestamps = append(imagesWithTimestamps, models.ImageWithTimestamp{
				URL:       images[0],
				Timestamp: promptForImage.ImagesPrompt[i].Duration,
			})
		}
	}

	fmt.Println("00000000000000000000000000000000000000000000000")

	return imagesWithTimestamps, nil
}
