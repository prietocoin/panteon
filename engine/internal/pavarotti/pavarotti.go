package pavarotti

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// Request es una petición de síntesis.
type Request struct {
	Text        string
	Voice       string
	DurationSec float64
}

// Synthesizer convierte texto en un WAV.
type Synthesizer interface {
	Synthesize(ctx context.Context, request Request) ([]byte, error)
}

// ToneSynth es el sintetizador offline del MVP. Genera una locución tonal
// determinista con la duración pedida, suficiente para validar el pipeline.
type ToneSynth struct {
	SampleRate int
}

// Synthesize implementa Synthesizer.
func (s ToneSynth) Synthesize(ctx context.Context, request Request) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rate := s.SampleRate
	if rate <= 0 {
		rate = 24000
	}
	duration := request.DurationSec
	if duration <= 0 {
		duration = EstimateDuration(request.Text)
	}
	if duration > 600 {
		return nil, errors.New("pavarotti: duración excesiva")
	}

	total := int(math.Round(duration * float64(rate)))
	if total <= 0 {
		return nil, errors.New("pavarotti: duración inválida")
	}

	hash := fnv.New32a()
	_, _ = hash.Write([]byte(request.Text + "|" + request.Voice))
	base := 150.0 + float64(hash.Sum32()%70)

	samples := make([]int16, total)
	fade := int(0.04 * float64(rate))
	for i := 0; i < total; i++ {
		t := float64(i) / float64(rate)
		envelope := 0.55 + 0.45*math.Sin(2*math.Pi*3.1*t)
		// Pausas suaves para simular sílabas.
		envelope *= 0.65 + 0.35*math.Sin(2*math.Pi*1.7*t+0.8)
		voice := math.Sin(2*math.Pi*base*t)*0.6 +
			math.Sin(2*math.Pi*base*2.02*t)*0.22 +
			math.Sin(2*math.Pi*base*3.01*t)*0.08
		sample := voice * envelope * 0.16
		if i < fade {
			sample *= float64(i) / float64(fade)
		}
		if remaining := total - i; remaining < fade {
			sample *= float64(remaining) / float64(fade)
		}
		if sample > 0.95 {
			sample = 0.95
		}
		if sample < -0.95 {
			sample = -0.95
		}
		samples[i] = int16(sample * 32767)
	}
	return EncodeWAV(Audio{Samples: samples, Rate: rate, Channels: 1}), nil
}

// HTTPSynth conecta con un servicio TTS tipo Chatterbox/Pavarotti.
// Espera POST JSON {text, voice, format:"wav"} y una respuesta WAV.
type HTTPSynth struct {
	BaseURL string
	Path    string
	Client  *http.Client
}

type httpPayload struct {
	Text   string `json:"text"`
	Voice  string `json:"voice"`
	Format string `json:"format"`
}

// Synthesize implementa Synthesizer.
func (s HTTPSynth) Synthesize(ctx context.Context, request Request) ([]byte, error) {
	if strings.TrimSpace(s.BaseURL) == "" {
		return nil, errors.New("pavarotti: BaseURL vacía")
	}
	path := s.Path
	if path == "" {
		path = "/synthesize"
	}
	endpoint := strings.TrimRight(s.BaseURL, "/") + "/" + strings.TrimLeft(path, "/")

	payload, err := json.Marshal(httpPayload{
		Text:   request.Text,
		Voice:  request.Voice,
		Format: "wav",
	})
	if err != nil {
		return nil, fmt.Errorf("pavarotti: serializando petición: %w", err)
	}

	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("pavarotti: creando petición: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "audio/wav")

	response, err := client.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("pavarotti: llamando a %s: %w", endpoint, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("pavarotti: TTS respondió %s", response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, 50<<20))
	if err != nil {
		return nil, fmt.Errorf("pavarotti: leyendo audio: %w", err)
	}
	if _, err := DecodeWAV(body); err != nil {
		return nil, fmt.Errorf("pavarotti: respuesta WAV inválida: %w", err)
	}
	return body, nil
}
