package pavarotti_test

import (
	"context"
	"math"
	"testing"

	"elpanteon/engine/internal/pavarotti"
)

func TestToneSynthProducesWAV(t *testing.T) {
	body, err := (pavarotti.ToneSynth{}).Synthesize(context.Background(), pavarotti.Request{
		Text:        "hola mundo",
		DurationSec: 0.5,
	})
	if err != nil {
		t.Fatalf("Synthesize(): %v", err)
	}
	duration, err := pavarotti.Duration(body)
	if err != nil {
		t.Fatalf("Duration(): %v", err)
	}
	if math.Abs(duration-0.5) > 0.01 {
		t.Fatalf("duration = %v, want 0.5", duration)
	}
	audio, err := pavarotti.DecodeWAV(body)
	if err != nil {
		t.Fatalf("DecodeWAV(): %v", err)
	}
	if audio.Rate != 24000 || audio.Channels != 1 {
		t.Fatalf("audio = %d Hz, %d channels", audio.Rate, audio.Channels)
	}
}

func TestWAVRoundTrip(t *testing.T) {
	original := pavarotti.Audio{
		Samples:  []int16{0, 1000, -1000, 32767, -32768},
		Rate:     16000,
		Channels: 1,
	}
	body := pavarotti.EncodeWAV(original)
	decoded, err := pavarotti.DecodeWAV(body)
	if err != nil {
		t.Fatalf("DecodeWAV(): %v", err)
	}
	if decoded.Rate != original.Rate || len(decoded.Samples) != len(original.Samples) {
		t.Fatalf("decoded = %+v", decoded)
	}
	for i := range original.Samples {
		if decoded.Samples[i] != original.Samples[i] {
			t.Fatalf("sample %d = %d, want %d", i, decoded.Samples[i], original.Samples[i])
		}
	}
}

func TestMonoAtResamples(t *testing.T) {
	audio := pavarotti.Audio{
		Samples:  []int16{0, 16384, 0, -16384},
		Rate:     8000,
		Channels: 1,
	}
	mono := pavarotti.MonoAt(audio, 16000)
	if len(mono) != 8 {
		t.Fatalf("len = %d, want 8", len(mono))
	}
}

func TestEstimateDuration(t *testing.T) {
	if got := pavarotti.EstimateDuration("una frase corta"); got < 1.2 {
		t.Fatalf("duration = %v", got)
	}
}
