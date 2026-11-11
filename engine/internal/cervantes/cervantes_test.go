package cervantes_test

import (
	"context"
	"strings"
	"testing"

	"elpanteon/engine/internal/atlas"
	"elpanteon/engine/internal/cervantes"
	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/testutil"
)

func TestWriteBuildsScenesAndClips(t *testing.T) {
	dossier, err := (atlas.DeterministicBuilder{}).Build(context.Background(), testutil.Case())
	if err != nil {
		t.Fatalf("Build(): %v", err)
	}
	cut, err := cervantes.Write(dossier, cervantes.Options{ClipSeconds: 2})
	if err != nil {
		t.Fatalf("Write(): %v", err)
	}
	if err := cut.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	if len(cut.Scenes) != 4 {
		t.Fatalf("scenes = %d, want 4", len(cut.Scenes))
	}
	total := 0
	for _, scene := range cut.Scenes {
		total += len(scene.Clips)
		for _, clip := range scene.Clips {
			if clip.DurationSec != 2 {
				t.Fatalf("%s duration = %v", clip.ID, clip.DurationSec)
			}
			if strings.TrimSpace(clip.Narration) == "" {
				t.Fatalf("%s sin narración", clip.ID)
			}
		}
	}
	if total != 9 {
		t.Fatalf("clips = %d, want 9", total)
	}
}

func TestWriteUsesRepositoryCodeWhenAvailable(t *testing.T) {
	dossier, err := (atlas.DeterministicBuilder{}).Build(context.Background(), testutil.Case())
	if err != nil {
		t.Fatalf("Build(): %v", err)
	}
	dossier.Artifacts = append(dossier.Artifacts, contract.Artifact{
		ID:      "repo-code",
		Kind:    "code",
		Title:   "Código real",
		Content: "package main\n\nfunc main() {}",
		Source:  "https://github.com/acme/service",
	})
	cut, err := cervantes.Write(dossier, cervantes.Options{ClipSeconds: 1})
	if err != nil {
		t.Fatalf("Write(): %v", err)
	}
	total := 0
	found := false
	for _, scene := range cut.Scenes {
		for _, clip := range scene.Clips {
			total++
			if clip.Visual.Title == "Código del repositorio" {
				found = true
			}
		}
	}
	if total != 10 {
		t.Fatalf("clips = %d, want 10", total)
	}
	if !found {
		t.Fatal("falta el clip de código real")
	}
}
