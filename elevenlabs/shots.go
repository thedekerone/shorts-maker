package elevenlabs

import (
	"strings"

	"github.com/thedekerone/shorts-maker/models"
)

type Shot struct {
	Index      int
	Start, End float64
	Text       string // raw concatenated words in this shot
	Brief      string // short visual brief for LLM
}

type ShotPlan struct {
	TotalStart float64
	TotalEnd   float64
	Shots      []Shot
}

func BuildShotPlan(out *models.TranscriptionOutput, pauseGap float64, minDur, maxDur float64) ShotPlan {
	// Assumes a single segment; extend if you have multiple
	seg := out.Segments[0]
	words := seg.Words

	isSentenceEnd := func(w string) bool {
		if len(w) == 0 {
			return false
		}
		last := w[len(w)-1]
		return last == '.' || last == '!' || last == '?'
	}

	shots := []Shot{}
	cur := Shot{Index: 0, Start: words[0].Start}
	var buf []string

	flush := func(end float64) {
		if len(buf) == 0 {
			return
		}
		cur.End = end
		cur.Text = strings.Join(buf, " ")
		cur.Brief = makeBrief(cur.Text) // implement a tiny summarizer or leave blank for LLM
		shots = append(shots, cur)
		cur = Shot{Index: len(shots)}
		buf = buf[:0]
	}

	for i := 0; i < len(words); i++ {
		w := words[i]
		if cur.Start == 0 && len(shots) == 0 {
			cur.Start = w.Start
		}
		buf = append(buf, w.Word)

		// boundary by sentence end or pause
		var nextStart float64
		if i+1 < len(words) {
			nextStart = words[i+1].Start
		}
		longPause := i+1 < len(words) && (nextStart-w.End) >= pauseGap
		endByLen := (w.End - cur.Start) >= maxDur
		endBySentence := isSentenceEnd(w.Word)

		if endBySentence || longPause || endByLen {
			flush(w.End)
			if i+1 < len(words) {
				cur.Start = words[i+1].Start
			}
		}
	}
	if len(buf) > 0 {
		flush(words[len(words)-1].End)
	}

	// merge too-short shots forward when possible
	merged := []Shot{}
	for i := 0; i < len(shots); i++ {
		s := shots[i]
		if (s.End-s.Start) < minDur && i+1 < len(shots) {
			nxt := shots[i+1]
			s.End = nxt.End
			s.Text = s.Text + " " + nxt.Text
			s.Brief = makeBrief(s.Text)
			i++ // skip next
		}
		merged = append(merged, s)
	}

	return ShotPlan{
		TotalStart: seg.Start,
		TotalEnd:   seg.End,
		Shots:      merged,
	}
}

// Minimal brief maker: clamp length, keep nouns-ish words.
// Replace with a better keyphrase extractor if you like.
func makeBrief(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 180 {
		return s
	}
	// naive trim at word boundary
	r := []rune(s)
	if len(r) > 180 {
		r = r[:180]
	}
	out := string(r)
	if i := strings.LastIndex(out, " "); i > 0 {
		out = out[:i]
	}
	return out
}
