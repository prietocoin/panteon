package davinci_test

import (
	"os"
	"strings"
	"testing"

	"elpanteon/engine/internal/davinci"
	"elpanteon/engine/internal/galileo/contract"
)

func TestRenderClipWithBeats(t *testing.T) {
	clip := contract.Clip{
		ID:          "clip-beats",
		Narration:   "Narración de prueba.",
		DurationSec: 75,
		Visual: contract.Visual{
			Kind:  "beats",
			Title: "Escena",
			Beats: []contract.Beat{
				{Start: 0, End: 0.3, Kind: "title", Title: "Primer beat", Subtitle: "Subtítulo"},
				{Start: 0.3, End: 1, Kind: "bullets", Title: "Segundo beat", Bullets: []string{"Uno", "Dos"}},
			},
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
	content := string(body)
	if !strings.Contains(content, `class="beat"`) {
		t.Fatal("falta contenedor de beats")
	}
	if !strings.Contains(content, `data-start="0.3000"`) {
		t.Fatal("falta data-start del beat")
	}
	if !strings.Contains(content, `name="duration" content="75.000"`) {
		t.Fatal("falta la duración de la escena")
	}
}
