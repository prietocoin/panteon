package cervantes_test

import (
	"context"
	"testing"

	"elpanteon/engine/internal/atlas"
	"elpanteon/engine/internal/cervantes"
	"elpanteon/engine/internal/testutil"
)

func TestWriteSceneModeCreatesVariableDurations(t *testing.T) {
	dossier, err := (atlas.DeterministicBuilder{}).Build(context.Background(), testutil.Case())
	if err != nil {
		t.Fatalf("Build(): %v", err)
	}
	cut, err := cervantes.Write(dossier, cervantes.Options{SceneMode: true})
	if err != nil {
		t.Fatalf("Write(): %v", err)
	}
	if err := cut.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	if len(cut.Scenes) != 6 {
		t.Fatalf("scenes = %d, want 6", len(cut.Scenes))
	}
	seenDurations := map[float64]bool{}
	for _, scene := range cut.Scenes {
		if len(scene.Clips) != 1 {
			t.Fatalf("%s clips = %d, want 1", scene.ID, len(scene.Clips))
		}
		clip := scene.Clips[0]
		if clip.DurationSec <= 0 {
			t.Fatalf("%s duration = %v", clip.ID, clip.DurationSec)
		}
		if len(clip.Visual.Beats) == 0 {
			t.Fatalf("%s sin beats", clip.ID)
		}
		if len(clip.Narration) < 80 {
			t.Fatalf("%s narración demasiado corta: %d", clip.ID, len(clip.Narration))
		}
		seenDurations[clip.DurationSec] = true
	}
	if len(seenDurations) < 2 {
		t.Fatal("las escenas deberían tener duraciones distintas")
	}
}
