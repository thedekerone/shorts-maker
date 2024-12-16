package subtitles

type SubtitleStyles struct {
	FontFamily  string
	BorderColor string
	Color       string
	BorderWidth string
	FontSize    string
}

type Subtitle struct {
	Text      string
	Style     *SubtitleStyles
	StartTime float32
	EndTime   float32
}

func CreateSubtitle(text string, style *SubtitleStyles, startTime float32, endTime float32) Subtitle {
	subtitle := Subtitle{
		Text:      text,
		Style:     style,
		StartTime: startTime,
		EndTime:   endTime,
	}

	return subtitle
}
