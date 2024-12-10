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
	StartTime int
	EndTime   int
}

func CreateSubtitle(text string, style *SubtitleStyles, startTime int, endTime int) Subtitle {
	subtitle := Subtitle{
		Text:      text,
		Style:     style,
		StartTime: startTime,
		EndTime:   endTime,
	}

	return subtitle
}
