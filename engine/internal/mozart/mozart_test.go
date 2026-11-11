package mozart_test

import (
	"context"
	"math"
	"os"
	"testing"

	"elpanteon/engine/internal/mozart"
	"elpanteon/engine/internal/pavarotti"
)

func TestWAVMixerMixesNarrationAndMusic(t *testing.T) {
	first, err := (pavarotti.ToneSynth{}).Synthesize(context.Background(), pavarotti.Request{
		Text:        "primera",
		DurationSec: 0.5,
	})
	if err != nil {
		t.Fatalf("Synthesize(): %v", err)
	}
	second, err := (pavarotti.ToneSynth{}).Synthesize(context.Background(), pavarotti.Request{
		Text:        "segunda",
		DurationSec: 0.4,
	})
	if err != nil {
		t.Fatalf("Synthesize(): %v", err)
	}

	out := t.TempDir() + "/master.wav"
	err = (mozart.WAVMixer{}).Mix(context.Background(), []mozart.Track{
		{Data: first, OffsetSec: 0},
		{Data: second, OffsetSec: 0.5},
	}, 0.9, out)
	if err != nil {
		t.Fatalf("Mix(): %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile(): %v", err)
	}
	duration, err := pavarotti.Duration(body)
	if err != nil {
		t.Fatalf("Duration(): %v", err)
	}
	if math.Abs(duration-0.9) > 0.02 {
		t.Fatalf("duration = %v, want 0.9", duration)
	}
}
