package mozart

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"elpanteon/engine/internal/pavarotti"
)

// Track es una pista de audio posicionada en la línea de tiempo.
type Track struct {
	Data      []byte
	OffsetSec float64
	Gain      float64
}

type mixedTrack struct {
	samples   []float64
	offsetSec float64
}

// Mixer mezcla las locuciones con música de fondo y ducking.
type Mixer interface {
	Mix(ctx context.Context, tracks []Track, totalSec float64, outPath string) error
}

// WAVMixer es el mezclador offline en Go puro.
type WAVMixer struct {
	SampleRate  int
	MusicVolume float64
}

// Mix implementa Mixer.
func (m WAVMixer) Mix(ctx context.Context, tracks []Track, totalSec float64, outPath string) error {
	rate := m.SampleRate
	if rate <= 0 {
		rate = 24000
	}
	musicVolume := m.MusicVolume
	if musicVolume <= 0 {
		musicVolume = 1
	}

	decoded := make([]mixedTrack, 0, len(tracks))
	maxEnd := totalSec
	for _, track := range tracks {
		if err := ctx.Err(); err != nil {
			return err
		}
		audio, err := pavarotti.DecodeWAV(track.Data)
		if err != nil {
			return fmt.Errorf("mozart: decodificando pista: %w", err)
		}
		samples := pavarotti.MonoAt(audio, rate)
		if len(samples) == 0 {
			continue
		}
		gain := track.Gain
		if gain <= 0 {
			gain = 1
		}
		for i := range samples {
			samples[i] *= gain
		}
		decoded = append(decoded, mixedTrack{samples: samples, offsetSec: track.OffsetSec})
		end := track.OffsetSec + float64(len(samples))/float64(rate)
		if end > maxEnd {
			maxEnd = end
		}
	}

	totalSamples := int(math.Ceil(maxEnd * float64(rate)))
	if totalSamples <= 0 {
		return fmt.Errorf("mozart: duración total inválida")
	}
	voice := make([]float64, totalSamples)
	for _, track := range decoded {
		offset := int(math.Round(track.offsetSec * float64(rate)))
		if offset < 0 {
			offset = 0
		}
		for i, sample := range track.samples {
			position := offset + i
			if position >= totalSamples {
				break
			}
			voice[position] += sample
		}
	}

	out := make([]float64, totalSamples)
	for i := 0; i < totalSamples; i++ {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		t := float64(i) / float64(rate)
		music := backgroundMusic(t) * musicVolume
		duck := 1.0 / (1.0 + math.Abs(voice[i])*14.0)
		sample := voice[i] + music*duck
		out[i] = sample
	}

	for _, track := range tracks {
		offset := int(math.Round(track.OffsetSec * float64(rate)))
		addClick(out, offset, rate, 0.05)
	}

	samples := make([]int16, totalSamples)
	for i, sample := range out {
		if sample > 0.98 {
			sample = 0.98
		}
		if sample < -0.98 {
			sample = -0.98
		}
		samples[i] = int16(sample * 32767)
	}
	applyFade(samples, rate, 0.03)

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("mozart: creando directorio: %w", err)
	}
	body := pavarotti.EncodeWAV(pavarotti.Audio{Samples: samples, Rate: rate, Channels: 1})
	temporary := outPath + ".tmp"
	if err := os.WriteFile(temporary, body, 0o644); err != nil {
		return fmt.Errorf("mozart: escribiendo %s: %w", temporary, err)
	}
	if err := os.Rename(temporary, outPath); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("mozart: publicando %s: %w", outPath, err)
	}
	return nil
}

// backgroundMusic genera un pad de acordes lentos.
func backgroundMusic(t float64) float64 {
	chords := [][3]float64{
		{110.00, 164.81, 220.00},
		{130.81, 196.00, 261.63},
		{98.00, 146.83, 196.00},
		{146.83, 220.00, 293.66},
	}
	index := int(t/4.0) % len(chords)
	chord := chords[index]
	phase := math.Mod(t, 4.0)
	envelope := math.Min(1, phase/0.8) * math.Min(1, (4.0-phase)/0.8)
	if envelope < 0 {
		envelope = 0
	}
	lfo := 0.75 + 0.25*math.Sin(2*math.Pi*0.11*t)
	return (math.Sin(2*math.Pi*chord[0]*t)*0.5 +
		math.Sin(2*math.Pi*chord[1]*t)*0.3 +
		math.Sin(2*math.Pi*chord[2]*t)*0.2) * 0.035 * envelope * lfo
}

func addClick(buffer []float64, offset, rate int, amplitude float64) {
	if offset < 0 || offset >= len(buffer) {
		return
	}
	length := int(0.06 * float64(rate))
	for i := 0; i < length && offset+i < len(buffer); i++ {
		envelope := 1 - float64(i)/float64(length)
		buffer[offset+i] += math.Sin(2*math.Pi*720*float64(i)/float64(rate)) * amplitude * envelope
	}
}

func applyFade(samples []int16, rate int, seconds float64) {
	fade := int(seconds * float64(rate))
	if fade <= 0 || len(samples) == 0 {
		return
	}
	if fade*2 > len(samples) {
		fade = len(samples) / 2
	}
	for i := 0; i < fade; i++ {
		samples[i] = int16(float64(samples[i]) * float64(i) / float64(fade))
		last := len(samples) - 1 - i
		samples[last] = int16(float64(samples[last]) * float64(i) / float64(fade))
	}
}
