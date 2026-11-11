package encoder_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"elpanteon/engine/internal/davinci"
	"elpanteon/engine/internal/encoder"
	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/pixar"
)

func TestFFmpegEncoderWritesMP4(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "tools", "ffmpeg", "ffmpeg.exe")
	if _, err := os.Stat(binary); err != nil {
		t.Skip("FFmpeg portable no disponible")
	}

	clip := contract.Clip{
		ID:          "clip-mp4",
		DurationSec: 0.5,
		Visual:      contract.Visual{Kind: "title", Title: "MP4", Subtitle: "Prueba"},
	}
	dir := t.TempDir()
	frames, err := pixar.NewRaster().Render(context.Background(), clip, davinci.Page{ClipID: clip.ID}, pixar.Options{
		Width:  160,
		Height: 90,
		FPS:    4,
		OutDir: dir,
	})
	if err != nil {
		t.Fatalf("Render(): %v", err)
	}

	path, err := (encoder.FFmpegEncoder{Binary: binary}).Encode(
		context.Background(),
		frames,
		"",
		filepath.Join(dir, "master"),
		4,
	)
	if err != nil {
		t.Fatalf("Encode(): %v", err)
	}
	if filepath.Ext(path) != ".mp4" {
		t.Fatalf("ext = %q", filepath.Ext(path))
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(): %v", err)
	}
	if len(body) < 16 || !bytes.Contains(body[:16], []byte("ftyp")) {
		t.Fatalf("archivo MP4 inválido (%d bytes)", len(body))
	}
}
