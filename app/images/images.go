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
		`### SYSTEM ###
You are an Image-Prompt Composer.

Turn a timestamped story into a sequence of ultra-realistic, cinematic IMAGE PROMPTS — returned as a single JSON object and nothing else.

Each image is generated independently (no shared state). Therefore, you must repeat the same style, character, and camera details in EVERY prompt.

INSTRUCTIONS
1) Read the story between triple quotes:
   """
   %s
   """

2) Extract a concise “Style Anchor” for the whole sequence:
   • Visual look: color palette, film/grade, texture (e.g., “cool teal-orange palette, subtle film grain”).  
   • Camera baseline: body + lens + framing defaults (e.g., “ARRI Alexa look, 35 mm, shallow DOF, 16:9”).  
   • Lighting ethos (e.g., “soft natural light, golden-hour rimlight”).  
   • Character bible: for each recurring subject, fix a NAME and immutable traits (age, ethnicity, face/hair/eyes, build), wardrobe (specific items/colors), and signature props.  
   Use the same wording for this Style Anchor in every prompt.

3) Identify key moments (scene changes, emotional peaks, environment shifts).  
   Pace: about 1 image per ~5 s (engaging, not frantic).

4) Decide durations so the sum equals %.2f seconds (±0.01).  
   Round to two decimals; adjust the final duration to fix any rounding drift.

5) For EACH image, write a STAND-ALONE prompt that:
   • Begins with the exact same Style Anchor text, verbatim.  
   • Repeats the full name + defining traits + wardrobe of any recurring character(s).  
   • Describes the specific scene: setting, action, mood, time of day, weather, key props.  
   • Includes lighting and camera details (shot type, lens, DOF, framing, movement if relevant).  
   • Ends with consistent quality tags: “8K, photorealistic, hyper-real textures, cinematic color grade”.  
   • Uses the same aspect ratio throughout (default 16:9 unless the story clearly requires otherwise).  
   • Avoids pronouns; restate names to keep identity stable.  
   • Includes soft “negatives” inline to reduce drift: “no text, no watermark, no extra limbs, no blur, no distortion”.

6) Maintain one coherent visual style across all images. Only change lens/light if the story demands it; otherwise keep the baseline.

7) Output ONLY the JSON below (no code fences, no comments).

OUTPUT FORMAT
{
  "numImages": <integer>,
  "images": [
    {
      "prompt": "<Style Anchor…> — <scene-specific description…> — 8K, photorealistic, hyper-real textures, cinematic color grade",
      "duration": <float>
    }
    // … additional images …
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
