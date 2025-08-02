package pkg

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thedekerone/shorts-maker/models"
)

// tiltEffect is an ASS override tag that applies subtle scaling animation.
const tiltEffect = "{\\fscx50\\fscy50\\t(0,60,\\fscx55\\fscy55)\\t(60,140,\\fscx50\\fscy50)}"

// CreateDialogFromWords converts a speech segment into individual ASS dialogue
// lines—one per word—so that each word can be animated independently.
func CreateDialogFromWords(seg models.Segment) (string, error) {
	if len(seg.Words) == 0 {
		return "", errors.New("segment contains no words")
	}

	var b strings.Builder

	for idx, w := range seg.Words {
		start := w.Start
		end := seg.End

		if idx == 0 {
			start = seg.Start
		}
		if idx < len(seg.Words)-1 {
			end = seg.Words[idx+1].Start
		}

		// Sanity checks
		if start < 0 {
			start = 0
		}
		if start > end {
			return "", fmt.Errorf("start time %.3f greater than end time %.3f", start, end)
		}

		fmt.Fprintf(&b,
			"Dialogue: 0,%s,%s,Default,,0000,0000,0000,,%s%s\n",
			toASSTimestamp(start),
			toASSTimestamp(end),
			tiltEffect,
			w.Word,
		)
	}

	return b.String(), nil
}

// CreateDialog produces a single ASS dialogue line for the entire segment.
func CreateDialog(seg models.Segment) string {
	return fmt.Sprintf(
		"\nDialogue: 0,%s,%s,Default,,0000,0000,0000,,%s",
		toASSTimestamp(seg.Start),
		toASSTimestamp(seg.End),
		joinWordsWithTiming(seg),
	)
}

func joinWordsWithTiming(seg models.Segment) string {
	var b strings.Builder

	for i, w := range seg.Words {
		var gap float64
		switch {
		case i == 0:
			gap = w.Start - seg.Start
		case i < len(seg.Words)-1:
			gap = seg.Words[i+1].Start - w.End
		}

		fmt.Fprintf(&b, "%s ", formatKTag(w, gap))
	}

	return strings.TrimSpace(b.String())
}

func formatKTag(w models.Word, offset float64) string {
	// \k expects centiseconds (1/100s) as an integer.
	durationCs := int((w.End - w.Start + offset) * 100)
	return fmt.Sprintf("{\\k%d}%s", durationCs, w.Word)
}

func toASSTimestamp(sec float64) string {
	d := time.Duration(sec * float64(time.Second))
	hrs := int(d.Hours())
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60
	cs := int(d.Milliseconds()/10) % 100
	return fmt.Sprintf("%d:%02d:%02d.%02d", hrs, mins, secs, cs)
}

// CreateAssFile renders an ASS subtitle file on disk by merging a base template
// with the generated dialogue lines.
func CreateAssFile(path string, tr models.TranscriptionOutput) error {
	base, err := os.ReadFile(filepath.Join("assets", "tilted.ass"))
	if err != nil {
		return fmt.Errorf("read base ASS file: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	if _, err = w.Write(base); err != nil {
		return fmt.Errorf("write base template: %w", err)
	}

	for _, seg := range tr.Segments {
		dlg, err := CreateDialogFromWords(seg)
		if err != nil {
			return fmt.Errorf("build dialog for segment: %w", err)
		}
		if _, err = w.WriteString(dlg); err != nil {
			return fmt.Errorf("write dialog: %w", err)
		}
	}

	return w.Flush()
}
