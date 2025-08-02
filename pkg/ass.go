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

const (
	tiltEffect            = "{\\fscx50\\fscy50\\t(0,60,\\fscx55\\fscy55)\\t(60,140,\\fscx50\\fscy50)}"
	baseStyle             = "{\\blur0.6\\bord2\\shad2\\c&H00FFFF&\\t(0,200,\\c&HFFFFFF&)}"
	fadeEffect            = "\\fad(50,50)"
	defaultOverlapSeconds = 0.05
	punctExtensionSeconds = 0.25
	punctuationRunes      = ",.!?"
)

var ErrEmptySegment = errors.New("segment contains no words")

func BuildDialogLines(seg models.Segment) (string, error) {
	if len(seg.Words) == 0 {
		return "", ErrEmptySegment
	}

	var b strings.Builder
	groups := groupWords(seg.Words)

	for gi, grp := range groups {
		start := grp[0].Start
		end := seg.End
		if gi < len(groups)-1 {
			end = groups[gi+1][0].Start + defaultOverlapSeconds
		}
		if containsPunctuation(grp[len(grp)-1].Word) {
			end += punctExtensionSeconds
		}
		if start < 0 {
			start = 0
		}
		if start > end {
			return "", fmt.Errorf("inconsistent timing: start=%.3f > end=%.3f", start, end)
		}
		fmt.Fprintf(&b,
			"Dialogue: 0,%s,%s,Default,,0000,0000,0000,,%s%s%s%s\n",
			toASSTimestamp(start),
			toASSTimestamp(end),
			tiltEffect,
			baseStyle,
			fadeEffect,
			joinGroupWords(grp),
		)
	}
	return b.String(), nil
}

func BuildSingleDialogLine(seg models.Segment) string {
	return fmt.Sprintf(
		"Dialogue: 0,%s,%s,Default,,0000,0000,0000,,%s\n",
		toASSTimestamp(seg.Start),
		toASSTimestamp(seg.End),
		joinWordsWithTiming(seg),
	)
}

func CreateASSFile(dstPath, templateDir string, tr models.TranscriptionOutput) error {
	headerPath := filepath.Join(templateDir, "tilted.ass")
	header, err := os.ReadFile(headerPath)
	if err != nil {
		return fmt.Errorf("read template %q: %w", headerPath, err)
	}

	f, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create %q: %w", dstPath, err)
	}
	defer f.Close()

	if _, err = f.Write(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	bw := bufio.NewWriter(f)
	for _, seg := range tr.Segments {
		lines, err := BuildDialogLines(seg)
		if err != nil {
			return fmt.Errorf("segment at %.2fs: %w", seg.Start, err)
		}
		if _, err = bw.WriteString(lines); err != nil {
			return fmt.Errorf("write dialogue: %w", err)
		}
	}
	return bw.Flush()
}

func groupWords(words []models.Word) [][]models.Word {
	var groups [][]models.Word
	var buf []models.Word
	for _, w := range words {
		buf = append(buf, w)
		if len(w.Word) > 2 || containsPunctuation(w.Word) {
			groups = append(groups, buf)
			buf = nil
		}
	}
	if len(buf) != 0 {
		groups = append(groups, buf)
	}
	return groups
}

func joinGroupWords(words []models.Word) string {
	parts := make([]string, len(words))
	for i, w := range words {
		parts[i] = w.Word
	}
	return strings.Join(parts, " ")
}

func joinWordsWithTiming(seg models.Segment) string {
	var b strings.Builder
	for i, w := range seg.Words {
		gap := leadingGap(seg, i)
		fmt.Fprintf(&b, "%s ", buildKTag(w, gap))
	}
	return strings.TrimSpace(b.String())
}

func leadingGap(seg models.Segment, idx int) float64 {
	switch idx {
	case 0:
		return seg.Words[0].Start - seg.Start
	case len(seg.Words) - 1:
		return 0
	default:
		return seg.Words[idx+1].Start - seg.Words[idx].End
	}
}

func buildKTag(w models.Word, gap float64) string {
	dur := (w.End - w.Start) + gap
	if containsPunctuation(w.Word) {
		dur += punctExtensionSeconds
	}
	dur += float64(len(w.Word)) * 0.015
	centiseconds := int(dur * 100)
	return fmt.Sprintf("{\\k%d}%s", centiseconds, w.Word)
}

func containsPunctuation(s string) bool {
	return strings.ContainsAny(s, punctuationRunes)
}

func toASSTimestamp(sec float64) string {
	d := time.Duration(sec * float64(time.Second))
	totalSeconds := int(d.Seconds())
	hrs := totalSeconds / 3600
	mins := (totalSeconds % 3600) / 60
	secs := totalSeconds % 60
	centis := int(d.Milliseconds()/10) % 100
	return fmt.Sprintf("%d:%02d:%02d.%02d", hrs, mins, secs, centis)
}
