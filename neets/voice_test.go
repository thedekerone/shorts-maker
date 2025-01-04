package neets_test

import (
	"testing"

	"github.com/thedekerone/shorts-maker/neets"
)

func TestVoiceCall(t *testing.T) {
	n := neets.CreateNeets()

	text := `I work as a local radio announcer. One day, I got a creepy call from a girl.\n I work as the announcer for our town’s only radio station. In this town, we don’t have 911. The station acts as our unofficial emergency system.`
	vr := n.NewVoiceRequest(text,
		"https://api.neets.ai/v1/tts")

	s, err := vr.Call()

	if err != nil {
		t.Fatalf("%v", err)
	}
	t.Log(s)

}
