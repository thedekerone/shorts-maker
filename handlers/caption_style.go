package handlers

import (
	"strings"

	"github.com/thedekerone/shorts-maker/subtitles"
)

type captionStyleConfig struct {
	TemplatePath   string
	SubtitleStyles subtitles.SubtitleStyles
	TextPrefix     string
	Karaoke        bool
}

func resolveCaptionStyle(name string) captionStyleConfig {
	style := strings.TrimSpace(strings.ToLower(name))
	switch style {
	case "tilted", "tilted caption":
		return captionStyleConfig{
			TemplatePath: "handlers/assets/tilted.ass",
			SubtitleStyles: subtitles.SubtitleStyles{
				FontFamily:  "Roboto-Black",
				BorderColor: "black",
				Color:       "white",
				BorderWidth: 4,
				FontSize:    72,
			},
			TextPrefix: "{\\frz-4\\fscx45\\fscy45\\t(0,200,\\frz0)}",
			Karaoke:    true,
		}
	case "glow", "glow caption":
		return captionStyleConfig{
			TemplatePath: "handlers/assets/caption_glow.ass",
			SubtitleStyles: subtitles.SubtitleStyles{
				FontFamily:  "Montserrat-ExtraBold",
				BorderColor: "#FF8F1F",
				Color:       "white",
				BorderWidth: 6,
				FontSize:    70,
			},
			TextPrefix: "{\\bord3\\shad0}",
			Karaoke:    true,
		}
	case "highlight", "highlight caption":
		return captionStyleConfig{
			TemplatePath: "handlers/assets/caption_highlight.ass",
			SubtitleStyles: subtitles.SubtitleStyles{
				FontFamily:  "Montserrat-ExtraBold",
				BorderColor: "#FFD029",
				Color:       "white",
				BorderWidth: 5,
				FontSize:    68,
			},
			TextPrefix: "{\\bord2\\shad0}",
			Karaoke:    true,
		}
	case "background", "caption with background":
		return captionStyleConfig{
			TemplatePath: "handlers/assets/caption_background.ass",
			SubtitleStyles: subtitles.SubtitleStyles{
				FontFamily:  "Roboto-Black",
				BorderColor: "white",
				Color:       "white",
				BorderWidth: 0,
				FontSize:    70,
			},
			TextPrefix: "{\\bord0\\shad0}",
			Karaoke:    false,
		}
	default:
		return captionStyleConfig{
			TemplatePath: "handlers/assets/base.ass",
			SubtitleStyles: subtitles.SubtitleStyles{
				FontFamily:  "Roboto-Black",
				BorderColor: "black",
				Color:       "white",
				BorderWidth: 4,
				FontSize:    72,
			},
			TextPrefix: "{\\fscx40\\fscy40\\t(0,60,\\fscx45\\fscy45)\\t(60,140,\\fscx40\\fscy40)}",
			Karaoke:    false,
		}
	}
}
