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

// Visual effects for realism
const (
	tiltEffect     = "{\\fscx50\\fscy50\\t(0,60,\\fscx55\\fscy55)\\t(60,140,\\fscx50\\fscy50)}"
	baseStyle      = "{\\blur0.6\\bord2\\shad2\\c&H00FFFF&\\t(0,200,\\c&HFFFFFF&)}"
	fadeEffect     = "\\fad(50,50)" // 50ms fade in/out
	defaultOverlap = 0.05           // seconds overlap between words
	punctExtension = 0.25           // extend after punctuation
)

// CreateDialogFromWords generates more realistic ASS dialogue lines.
func CreateDialogFromWords(seg models.Segment) (string, error) {
	if len(seg.Words) == 0 {
		return "", errors.New("segment contains no words")
	}

	var b strings.Builder

	// Optional grouping of filler words
	wordGroups := groupWords(seg.Words)

	for _, group := range wordGroups {
		start := group[0].Start
		end := seg.End
		text := joinGroupWords(group)

		// Set end based on last word in group
		last := group[len(group)-1]
		if idx := indexOfWord(seg.Words, last); idx < len(seg.Words)-1 {
			end = seg.Words[idx+1].Start + defaultOverlap
		}

		// Adjust for punctuation pauses
		if strings.ContainsAny(last.Word, ",.!?") {
			end += punctExtension
		}

		// Sanity checks
		if start < 0 {
			start = 0
		}
		if start > end {
			return "", fmt.Errorf("start time %.3f greater than end time %.3f", start, end)
		}

		fmt.Fprintf(&b,
			"Dialogue: 0,%s,%s,Default,,0000,0000,0000,,%s%s%s%s\n",
			toASSTimestamp(start),
			toASSTimestamp(end),
			tiltEffect,
			baseStyle,
			fadeEffect,
			text,
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

// Group short filler words with the next longer one for smoother reading.
func groupWords(words []models.Word) [][]models.Word {
	var groups [][]models.Word
	buf := []models.Word{}
	for _, w := range words {
		buf = append(buf, w)
		if len(w.Word) > 2 || strings.ContainsAny(w.Word, ",.!?") {
			groups = append(groups, buf)
			buf = []models.Word{}
		}
	}
	if len(buf) > 0 {
		groups = append(groups, buf)
	}
	return groups
}

// Concatenate a group of words into one string
func joinGroupWords(words []models.Word) string {
	parts := make([]string, len(words))
	for i, w := range words {
		parts[i] = w.Word
	}
	return strings.Join(parts, " ")
}

// Helper to find a word’s index in the segment
func indexOfWord(all []models.Word, target models.Word) int {
	for i, w := range all {
		if w.Start == target.Start && w.End == target.End && w.Word == target.Word {
			return i
		}
	}
	return -1
}

// Karaoke-style timing for full-line subtitles
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
	duration := (w.End - w.Start) + offset

	// Extend slightly for punctuation
	if strings.ContainsAny(w.Word, ",.!?") {
		duration += punctExtension
	}

	// Scale duration with word length
	duration += float64(len(w.Word)) * 0.015

	durationCs := int(duration * 100) // centiseconds
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

// CreateAssFile writes a full ASS file combining template and generated dialogue.
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
