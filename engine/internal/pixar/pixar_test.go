package pixar_test

import (
	"context"
	"image/png"
	"os"
	"testing"

	"elpanteon/engine/internal/davinci"
	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/pixar"
)

func TestRasterRendersAllVisualKinds(t *testing.T) {
	cases := []contract.Visual{
		{Kind: "title", Title: "Título", Subtitle: "Subtítulo"},
		{Kind: "cards", Title: "Tarjetas", Cards: []contract.Card{{Title: "Uno", Body: "Cuerpo", Tone: "danger"}}},
		{Kind: "bullets", Title: "Viñetas", Bullets: []string{"Primera idea", "Segunda idea"}},
		{Kind: "code", Title: "Código", Code: "# comment\naction: value", CodeLanguage: "yaml"},
	}
	renderer := pixar.NewRaster()
	for _, visual := range cases {
		clip := contract.Clip{ID: "clip", DurationSec: 0.25, Visual: visual}
		page := davinci.Page{ClipID: "clip", Path: "clip.html"}
		paths, err := renderer.Render(context.Background(), clip, page, pixar.Options{
			Width:  320,
			Height: 180,
			FPS:    4,
			OutDir: t.TempDir(),
		})
		if err != nil {
			t.Fatalf("Render(%s): %v", visual.Kind, err)
		}
		if len(paths) != 1 {
			t.Fatalf("%s frames = %d", visual.Kind, len(paths))
		}
		file, err := os.Open(paths[0])
		if err != nil {
			t.Fatalf("Open(): %v", err)
		}
		img, err := png.Decode(file)
		file.Close()
		if err != nil {
			t.Fatalf("png.Decode(): %v", err)
		}
		if img.Bounds().Dx() != 320 || img.Bounds().Dy() != 180 {
			t.Fatalf("%s bounds = %v", visual.Kind, img.Bounds())
		}
	}
}

func TestChromiumRendersOneFrame(t *testing.T) {
	chrome := pixar.FindChrome()
	if chrome == "" {
		t.Skip("Chrome/Chromium no disponible")
	}
	clip := contract.Clip{
		ID:          "clip-chromium",
		DurationSec: 0.25,
		Visual:      contract.Visual{Kind: "title", Title: "Chromium", Subtitle: "Un frame"},
	}
	dir := t.TempDir()
	page, err := davinci.RenderClip(clip, dir, 320, 180)
	if err != nil {
		t.Fatalf("RenderClip(): %v", err)
	}
	paths, err := pixar.NewChromium(chrome).Render(context.Background(), clip, page, pixar.Options{
		Width:  320,
		Height: 180,
		FPS:    4,
		OutDir: dir,
	})
	if err != nil {
		t.Fatalf("Render(): %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("frames = %d, want 1", len(paths))
	}
	info, err := os.Stat(paths[0])
	if err != nil || info.Size() == 0 {
		t.Fatalf("frame inválido: %v", err)
	}
	file, err := os.Open(paths[0])
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	img, err := png.Decode(file)
	file.Close()
	if err != nil {
		t.Fatalf("png.Decode(): %v", err)
	}
	if img.Bounds().Dx() != 320 || img.Bounds().Dy() != 180 {
		t.Fatalf("bounds = %v, want 320x180", img.Bounds())
	}
}
