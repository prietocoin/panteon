package encoder_test

import (
	"context"
	"image/gif"
	"os"
	"path/filepath"
	"testing"

	"elpanteon/engine/internal/davinci"
	"elpanteon/engine/internal/encoder"
	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/pixar"
)

func TestGIFEncoderWritesAnimation(t *testing.T) {
	clip := contract.Clip{
		ID:          "clip-gif",
		DurationSec: 0.5,
		Visual:      contract.Visual{Kind: "title", Title: "GIF", Subtitle: "Prueba"},
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
	if len(frames) != 2 {
		t.Fatalf("frames = %d, want 2", len(frames))
	}

	path, err := (encoder.GIFEncoder{}).Encode(context.Background(), frames, "", filepath.Join(dir, "master"), 4)
	if err != nil {
		t.Fatalf("Encode(): %v", err)
	}
	if filepath.Ext(path) != ".gif" {
		t.Fatalf("ext = %q", filepath.Ext(path))
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	decoded, err := gif.DecodeAll(file)
	file.Close()
	if err != nil {
		t.Fatalf("DecodeAll(): %v", err)
	}
	if len(decoded.Image) != 2 {
		t.Fatalf("gif frames = %d, want 2", len(decoded.Image))
	}
	if decoded.Image[0].Bounds().Dx() != 160 {
		t.Fatalf("bounds = %v", decoded.Image[0].Bounds())
	}
}
