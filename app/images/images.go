package images

import (
	"fmt"

	"github.com/thedekerone/shorts-maker/elevenlabs"
	"github.com/thedekerone/shorts-maker/models"
	"github.com/thedekerone/shorts-maker/services"
)

func GetImagesWithTimestamps(shotPlan *elevenlabs.ShotPlan, mode string) ([]models.ImageWithTimestamp, error) {
	rs, err := services.NewReplicateService()
	deepseek, err := services.NewDeepSeekService()
	if err != nil {
		return nil, fmt.Errorf("error creating replicate service: %w", err)
	}

	totalDuration := shotPlan.TotalEnd

	var imagesWithTimestamps []models.ImageWithTimestamp

	var shotStrings string

	for _, v := range shotPlan.Shots {
		shotStrings = shotStrings + fmt.Sprintf("{ shot: %s, start: %.3f, end: %.3f } \n", v.Text, v.Start, v.End)
	}

	imageGenerationPrompts := fmt.Sprintf(
		`### SYSTEM ###
You are an Image-Prompt Composer.

Turn a timestamped story into a sequence of ultra-realistic, cinematic IMAGE PROMPTS — returned as a single JSON object and nothing else.

Each image is generated independently (no shared state). To keep the look cohesive, you MUST repeat the same Style Anchor and character details in EVERY prompt.

HARD RULES
• Do NOT mention cameras, lenses, focal lengths, apertures, ISO, shutter speed, or depth-of-field.  
• Keep one consistent visual design across all images (art direction, palette, lighting ethos, texture, grain, aspect ratio).  
• Use clear section labels and simple, declarative phrasing so the generator parses reliably.  
• Avoid pronouns for recurring subjects; restate names and traits every time.

INSTRUCTIONS
1) Read the story between triple quotes, this comes with the text(part of the story) of the image to generate as well as the start and end of that part of the story:
   """
   %s
   """

2) Detect the story’s **MOOD & GENRE** (e.g., hopeful, melancholic, tense; drama, thriller, adventure, romance).  
   Choose a matching **STYLE FAMILY** (e.g., “gritty neo-noir”, “warm nostalgic drama”, “cold techno-thriller”, “sun-bleached road movie”) that best supports that mood.

3) Create a single **STYLE ANCHOR** for the whole sequence. This exact text must be copied verbatim at the start of EVERY prompt:  
   • Art direction: “photorealistic, cinematic, natural materials, subtle film grain”.  
   • Style family (from step 2) and why it fits the mood (short phrase).  
   • Color palette: fixed hues/accents (e.g., “teal and amber highlights, muted neutrals”).  
   • Lighting ethos: general terms only (e.g., “soft directional sunlight with gentle rimlight” or “overcast diffuse light”).  
   • Aspect ratio: “16:9” (use a different ratio only if the story clearly demands it).  
   • Texture: “hyper-real surface detail, clean edges”.  
   • Negatives: “no text, no watermark, no logo, no extra fingers, normal human anatomy, no motion blur, no distortion”.

4) Build **CHARACTER SHEETS** for each recurring subject. Fix a NAME and immutable traits: age, ethnicity, facial structure, hair, eyes, build, wardrobe (specific items/colors), signature props.  
   Use the SAME wording for these traits every time that character appears.

5) Identify key moments (scene changes, emotional peaks, environment shifts).  
   Pacing: roughly 1 image per ~6-8 seconds (engaging, not frantic). Use timestamps if provided.

6) Assign **durations** so the total equals **%.2f** seconds (±0.01). Round to two decimals; adjust the final item to correct any rounding drift.

7) For EACH image, write a **stand-alone prompt** with the following consistent sections (labels included in the text):  
   • STYLE ANCHOR: <paste the exact Style Anchor text verbatim>  
   • CHARACTERS: <repeat full name + fixed traits + wardrobe for all visible recurring characters>  
   • SCENE: <setting, time of day, weather, key props, physical actions, objective description; no metaphors>  
   • LIGHTING: <use the lighting ethos; note direction/quality/intensity without technical camera terms>  
   • COMPOSITION: <framing language only: “wide establishing view”, “medium two-shot”, “tight portrait framing”, “over-shoulder”, “low angle”, “symmetrical composition”>  
   • MOOD CUES: <single sentence that states the emotional tone plainly>  
   • QUALITY TAGS: “8K, photorealistic, hyper-real textures, cinematic color grade”  
   • NEGATIVES: <repeat negatives from the Style Anchor>

8) Output ONLY the JSON below (no code fences, no comments).

OUTPUT FORMAT
{
  "numImages": <integer>,
  "images": [
    {
      "prompt": "<STYLE ANCHOR: …> <CHARACTERS: …> <SCENE: …> <LIGHTING: …> <COMPOSITION: …> <MOOD CUES: …> <QUALITY TAGS: …> <NEGATIVES: …>",
      "duration": <float>
    }
    // … additional images …
  ]
}
`, fmt.Sprintf("\n %v", shotStrings), totalDuration)

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
