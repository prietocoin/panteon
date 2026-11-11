package cervantes_test

import (
	"context"
	"testing"

	"elpanteon/engine/internal/atlas"
	"elpanteon/engine/internal/cervantes"
	"elpanteon/engine/internal/testutil"
)

func TestWriteExpandsCopiesForLoadTests(t *testing.T) {
	dossier, err := (atlas.DeterministicBuilder{}).Build(context.Background(), testutil.Case())
	if err != nil {
		t.Fatalf("Build(): %v", err)
	}
	cut, err := cervantes.Write(dossier, cervantes.Options{ClipSeconds: 20, Copies: 4})
	if err != nil {
		t.Fatalf("Write(): %v", err)
	}
	if err := cut.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}

	total := 0
	seen := map[string]bool{}
	for _, scene := range cut.Scenes {
		for _, clip := range scene.Clips {
			total++
			if seen[clip.ID] {
				t.Fatalf("id duplicado: %s", clip.ID)
			}
			seen[clip.ID] = true
			if clip.DurationSec != 20 {
				t.Fatalf("%s duration = %v", clip.ID, clip.DurationSec)
			}
		}
	}
	if total != 36 {
		t.Fatalf("clips = %d, want 36", total)
	}
	if len(cut.Scenes) != 16 {
		t.Fatalf("scenes = %d, want 16", len(cut.Scenes))
	}
}
