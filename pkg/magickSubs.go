package pkg

import (
	"fmt"
	"gopkg.in/gographics/imagick.v2/imagick"
)

// SubtitleConfig holds configuration for subtitle generation
type SubtitleConfig struct {
	Text            string
	Color           string
	BorderColor     string
	BackgroundColor string
	Font            string
	FontSize        float64
	Width           uint
	Height          uint
	ShadowRadius    float64
	ShadowSigma     float64
	ShadowOffsetX   int
	ShadowOffsetY   int
}

// DefaultConfig returns a SubtitleConfig with sensible defaults
func DefaultConfig() SubtitleConfig {
	return SubtitleConfig{
		Text:            "Sample Text",
		Color:           "white",
		BorderColor:     "black",
		BackgroundColor: "none",
		Font:            "Verdana-Bold-Italic",
		FontSize:        64,
		Width:           750,
		Height:          100,
		ShadowRadius:    70,
		ShadowSigma:     4,
		ShadowOffsetX:   5,
		ShadowOffsetY:   5,
	}
}

// Subtitle represents a subtitle generator
type Subtitle struct {
	config SubtitleConfig
	mw     *imagick.MagickWand
	dw     *imagick.DrawingWand
	pw     *imagick.PixelWand
}

// NewSubtitle creates a new Subtitle instance with the given configuration
func NewSubtitle(config SubtitleConfig) (*Subtitle, error) {
	imagick.Initialize()

	sub := &Subtitle{
		config: config,
		mw:     imagick.NewMagickWand(),
		dw:     imagick.NewDrawingWand(),
		pw:     imagick.NewPixelWand(),
	}

	return sub, nil
}

// Cleanup releases resources
func (s *Subtitle) Cleanup() {
	if s.mw != nil {
		s.mw.Destroy()
	}
	if s.dw != nil {
		s.dw.Destroy()
	}
	if s.pw != nil {
		s.pw.Destroy()
	}
	imagick.Terminate()
}

// Generate creates a subtitle image with the current configuration
func (s *Subtitle) Generate(outputPath string) error {
	defer s.Cleanup()

	// Create initial transparent image
	s.pw.SetColor(s.config.BackgroundColor)
	err := s.mw.NewImage(s.config.Width, s.config.Height, s.pw)
	if err != nil {
		return fmt.Errorf("failed to create new image: %v", err)
	}

	// Configure text properties
	s.pw.SetColor(s.config.Color)
	s.dw.SetFillColor(s.pw)
	s.dw.SetFont(s.config.Font)
	s.dw.SetFontSize(s.config.FontSize)

	s.pw.SetColor(s.config.BorderColor)
	s.dw.SetStrokeColor(s.pw)
	s.dw.SetTextAntialias(true)

	// Draw text
	s.dw.Annotation(25, 65, s.config.Text)

	// Apply text to image
	err = s.mw.DrawImage(s.dw)
	if err != nil {
		return fmt.Errorf("failed to draw image: %v", err)
	}

	// Trim excess space
	err = s.mw.TrimImage(0)
	if err != nil {
		return fmt.Errorf("failed to trim image: %v", err)
	}

	err = s.mw.ResetImagePage("")
	if err != nil {
		return fmt.Errorf("failed to reset image page: %v", err)
	}

	// Create shadow effect
	cw := s.mw.Clone()
	s.pw.SetColor("none")
	s.mw.SetImageBackgroundColor(s.pw)

	err = s.mw.ShadowImage(
		s.config.ShadowRadius,
		s.config.ShadowSigma,
		s.config.ShadowOffsetX,
		s.config.ShadowOffsetY,
	)
	if err != nil {
		return fmt.Errorf("failed to create shadow: %v", err)
	}

	// Composite original text over shadow
	err = s.mw.CompositeImage(cw, imagick.COMPOSITE_OP_OVER, 5, 5)
	if err != nil {
		return fmt.Errorf("failed to composite image: %v", err)
	}
	cw.Destroy()

	// Create final image with transparent background
	cw = imagick.NewMagickWand()
	cw.NewImage(s.mw.GetImageWidth(), s.mw.GetImageHeight(), s.pw)
	cw.CompositeImage(s.mw, imagick.COMPOSITE_OP_OVER, 0, 0)

	// Save the result
	err = cw.WriteImage(outputPath)
	if err != nil {
		return fmt.Errorf("failed to write image: %v", err)
	}

	return nil
}

// UpdateConfig updates the subtitle configuration
func (s *Subtitle) UpdateConfig(config SubtitleConfig) {
	s.config = config
}
