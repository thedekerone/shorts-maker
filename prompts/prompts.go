package prompts

import _ "embed"

var (
	//go:embed image_system_prompt.md
	imageSystemPrompt string

	//go:embed clip_system_prompt.md
	clipSystemPrompt string
)

func ImageSystemPrompt() string {
	return imageSystemPrompt
}

func ClipSystemPrompt() string {
	return clipSystemPrompt
}
