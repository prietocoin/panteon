package extract_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"elpanteon/engine/internal/galileo/extract"
	"elpanteon/engine/internal/galileo/source"
)

func loadFixture(t *testing.T) (source.Document, extract.Draft) {
	t.Helper()
	path := filepath.Join("..", "testdata", "postmortem.md")
	doc, err := source.FromFile(path, source.Options{})
	if err != nil {
		t.Fatalf("FromFile(): %v", err)
	}
	draft, err := (extract.MarkedExtractor{}).Extract(context.Background(), doc)
	if err != nil {
		t.Fatalf("Extract(): %v", err)
	}
	return doc, draft
}

func TestMarkedExtractorBuildsTheTriad(t *testing.T) {
	_, draft := loadFixture(t)

	if !strings.Contains(draft.Symptom.Summary, "503") {
		t.Fatalf("symptom = %q", draft.Symptom.Summary)
	}
	if !draft.Symptom.Observable {
		t.Fatal("el síntoma debía ser observable")
	}
	if !strings.Contains(draft.Cause.Summary, "pool de conexiones") {
		t.Fatalf("cause = %q", draft.Cause.Summary)
	}
	if len(draft.Solution.Actions) != 2 {
		t.Fatalf("actions = %d, want 2", len(draft.Solution.Actions))
	}
	if len(draft.Solution.Tradeoffs) != 1 {
		t.Fatalf("tradeoffs = %d, want 1", len(draft.Solution.Tradeoffs))
	}
	if !strings.Contains(draft.Solution.Verification, "72 horas") {
		t.Fatalf("verification = %q", draft.Solution.Verification)
	}
	if len(draft.Symptom.Evidence) == 0 || len(draft.Cause.Evidence) == 0 || len(draft.Solution.Evidence) == 0 {
		t.Fatal("la tríada debe tener evidencia")
	}
	if got := draft.Symptom.Evidence[0].Locator; !strings.HasPrefix(got, "line:") {
		t.Fatalf("locator = %q", got)
	}
}

func TestMarkedExtractorSupportsSpanishHeadings(t *testing.T) {
	text := `## Síntoma
La API devolvió errores 500.

## Causa raíz
El disco se llenó por logs sin rotación.

## Solución
Se activó rotación de logs.

## Compensaciones
- Se pierde historial detallado.

## Verificación
Los errores 500 desaparecieron durante 24 horas.
`
	doc := source.FromText(text, source.Options{})
	draft, err := (extract.MarkedExtractor{}).Extract(context.Background(), doc)
	if err != nil {
		t.Fatalf("Extract(): %v", err)
	}
	if !strings.Contains(draft.Solution.Summary, "rotación") {
		t.Fatalf("solution = %q", draft.Solution.Summary)
	}
	if len(draft.Solution.Tradeoffs) != 1 {
		t.Fatalf("tradeoffs = %d", len(draft.Solution.Tradeoffs))
	}
}

func TestMarkedExtractorReportsMissingSections(t *testing.T) {
	doc := source.FromText("Solo hay una descripción sin secciones.", source.Options{})
	_, err := (extract.MarkedExtractor{}).Extract(context.Background(), doc)
	if !errors.Is(err, extract.ErrMissingSections) {
		t.Fatalf("err = %v, want ErrMissingSections", err)
	}
}
