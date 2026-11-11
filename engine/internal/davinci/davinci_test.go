package davinci_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"elpanteon/engine/internal/atlas"
	"elpanteon/engine/internal/cervantes"
	"elpanteon/engine/internal/davinci"
	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/testutil"
)

func TestRenderClipWritesProceduralHTML(t *testing.T) {
	dossier, err := (atlas.DeterministicBuilder{}).Build(context.Background(), testutil.Case())
	if err != nil {
		t.Fatalf("Build(): %v", err)
	}
	cut, err := cervantes.Write(dossier, cervantes.Options{ClipSeconds: 1})
	if err != nil {
		t.Fatalf("Write(): %v", err)
	}

	for _, scene := range cut.Scenes {
		for _, clip := range scene.Clips {
			page, err := davinci.RenderClip(clip, t.TempDir(), 640, 360)
			if err != nil {
				t.Fatalf("RenderClip(%s): %v", clip.ID, err)
			}
			body, err := os.ReadFile(page.Path)
			if err != nil {
				t.Fatalf("ReadFile(): %v", err)
			}
			if !strings.Contains(string(body), "<!DOCTYPE html>") {
				t.Fatalf("%s no es HTML", clip.ID)
			}
			if !strings.Contains(string(body), "window.setFrame") {
				t.Fatalf("%s no inyecta el reloj virtual", clip.ID)
			}
		}
	}
}

func TestRenderClipEscapesContent(t *testing.T) {
	clip := contract.Clip{
		ID:          "clip-xss",
		DurationSec: 1,
		Visual: contract.Visual{
			Kind:  "title",
			Title: `<script>alert("x")</script>`,
		},
	}
	page, err := davinci.RenderClip(clip, t.TempDir(), 640, 360)
	if err != nil {
		t.Fatalf("RenderClip(): %v", err)
	}
	body, err := os.ReadFile(page.Path)
	if err != nil {
		t.Fatalf("ReadFile(): %v", err)
	}
	if strings.Contains(string(body), `<script>alert("x")</script>`) {
		t.Fatal("el contenido no fue escapado")
	}
	if !strings.Contains(string(body), "&lt;script&gt;") {
		t.Fatal("falta el contenido escapado")
	}
	_ = filepath.Base(page.Path)
}
