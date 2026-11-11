package atlas_test

import (
	"context"
	"strings"
	"testing"

	"elpanteon/engine/internal/atlas"
	"elpanteon/engine/internal/testutil"
)

func TestDeterministicBuilderCreatesDossier(t *testing.T) {
	dossier, err := (atlas.DeterministicBuilder{}).Build(context.Background(), testutil.Case())
	if err != nil {
		t.Fatalf("Build(): %v", err)
	}
	if err := dossier.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	if len(dossier.Facts) != 5 {
		t.Fatalf("facts = %d, want 5", len(dossier.Facts))
	}
	if len(dossier.Artifacts) != 4 {
		t.Fatalf("artifacts = %d, want 4", len(dossier.Artifacts))
	}
	if !strings.Contains(dossier.Summary, "Síntoma:") {
		t.Fatalf("summary = %q", dossier.Summary)
	}
	code := ""
	for _, artifact := range dossier.Artifacts {
		if artifact.ID == "solution-code" {
			code = artifact.Content
		}
	}
	if !strings.Contains(code, "Limitar el pool") {
		t.Fatalf("code artifact = %q", code)
	}
}

func TestDeterministicBuilderRejectsInvalidCase(t *testing.T) {
	engineeringCase := testutil.Case()
	engineeringCase.Symptom.Evidence = nil
	if _, err := (atlas.DeterministicBuilder{}).Build(context.Background(), engineeringCase); err == nil {
		t.Fatal("Build() debía fallar con un caso inválido")
	}
}
