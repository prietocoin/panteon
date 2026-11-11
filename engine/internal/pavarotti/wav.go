package pavarotti

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
)

// Audio es PCM de 16 bits.
type Audio struct {
	Samples  []int16
	Rate     int
	Channels int
}

// EncodeWAV serializa PCM 16-bit a un WAV RIFF.
func EncodeWAV(audio Audio) []byte {
	if audio.Rate <= 0 {
		audio.Rate = 24000
	}
	if audio.Channels <= 0 {
		audio.Channels = 1
	}
	dataSize := len(audio.Samples) * 2
	byteRate := audio.Rate * audio.Channels * 2
	blockAlign := audio.Channels * 2

	buffer := bytes.NewBuffer(make([]byte, 0, 44+dataSize))
	buffer.WriteString("RIFF")
	binary.Write(buffer, binary.LittleEndian, uint32(36+dataSize))
	buffer.WriteString("WAVE")
	buffer.WriteString("fmt ")
	binary.Write(buffer, binary.LittleEndian, uint32(16))
	binary.Write(buffer, binary.LittleEndian, uint16(1))
	binary.Write(buffer, binary.LittleEndian, uint16(audio.Channels))
	binary.Write(buffer, binary.LittleEndian, uint32(audio.Rate))
	binary.Write(buffer, binary.LittleEndian, uint32(byteRate))
	binary.Write(buffer, binary.LittleEndian, uint16(blockAlign))
	binary.Write(buffer, binary.LittleEndian, uint16(16))
	buffer.WriteString("data")
	binary.Write(buffer, binary.LittleEndian, uint32(dataSize))
	for _, sample := range audio.Samples {
		binary.Write(buffer, binary.LittleEndian, sample)
	}
	return buffer.Bytes()
}

// DecodeWAV lee PCM 16-bit mono o estéreo.
func DecodeWAV(data []byte) (Audio, error) {
	if len(data) < 44 {
		return Audio{}, errors.New("wav: archivo demasiado corto")
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return Audio{}, errors.New("wav: cabecera RIFF/WAVE inválida")
	}

	var (
		format     uint16
		channels   uint16
		sampleRate uint32
		bits       uint16
		payload    []byte
	)

	offset := 12
	for offset+8 <= len(data) {
		id := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		if offset+size > len(data) {
			size = len(data) - offset
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return Audio{}, errors.New("wav: chunk fmt incompleto")
			}
			format = binary.LittleEndian.Uint16(data[offset : offset+2])
			channels = binary.LittleEndian.Uint16(data[offset+2 : offset+4])
			sampleRate = binary.LittleEndian.Uint32(data[offset+4 : offset+8])
			bits = binary.LittleEndian.Uint16(data[offset+14 : offset+16])
		case "data":
			payload = data[offset : offset+size]
		}
		offset += size
		if size%2 == 1 {
			offset++
		}
	}

	if format != 1 || bits != 16 {
		return Audio{}, fmt.Errorf("wav: solo se soporta PCM 16-bit (format=%d bits=%d)", format, bits)
	}
	if channels == 0 || sampleRate == 0 {
		return Audio{}, errors.New("wav: fmt sin canales o sample rate")
	}
	if len(payload)%2 == 1 {
		payload = payload[:len(payload)-1]
	}

	samples := make([]int16, len(payload)/2)
	for i := range samples {
		samples[i] = int16(binary.LittleEndian.Uint16(payload[i*2 : i*2+2]))
	}
	return Audio{Samples: samples, Rate: int(sampleRate), Channels: int(channels)}, nil
}

// Duration devuelve la duración de un WAV.
func Duration(data []byte) (float64, error) {
	audio, err := DecodeWAV(data)
	if err != nil {
		return 0, err
	}
	frames := len(audio.Samples) / audio.Channels
	return float64(frames) / float64(audio.Rate), nil
}

// MonoAt convierte a mono y remuestrea por vecino más cercano.
func MonoAt(audio Audio, targetRate int) []float64 {
	if targetRate <= 0 {
		targetRate = 24000
	}
	if audio.Channels <= 0 || audio.Rate <= 0 {
		return nil
	}
	sourceFrames := len(audio.Samples) / audio.Channels
	if sourceFrames == 0 {
		return nil
	}
	targetFrames := int(math.Round(float64(sourceFrames) * float64(targetRate) / float64(audio.Rate)))
	if targetFrames <= 0 {
		return nil
	}
	out := make([]float64, targetFrames)
	for i := range out {
		source := int(float64(i) * float64(audio.Rate) / float64(targetRate))
		if source >= sourceFrames {
			source = sourceFrames - 1
		}
		var sum int32
		for channel := 0; channel < audio.Channels; channel++ {
			sum += int32(audio.Samples[source*audio.Channels+channel])
		}
		out[i] = float64(sum) / float64(audio.Channels) / 32768.0
	}
	return out
}

// EstimateDuration estima la locución a 2.7 palabras por segundo.
func EstimateDuration(text string) float64 {
	words := len(strings.Fields(text))
	if words == 0 {
		return 1.5
	}
	seconds := float64(words)/2.7 + 0.4
	if seconds < 1.2 {
		seconds = 1.2
	}
	return seconds
}
