package extract_test

import (
	"context"
	"testing"

	"elpanteon/engine/internal/galileo/extract"
	"elpanteon/engine/internal/galileo/source"
)

func TestMarkedExtractorSeparatesHTMLLists(t *testing.T) {
	html := `<h2>Symptom</h2><p>La API devolvió 503 durante 8 minutos.</p>
<h2>Cause</h2><p>El pool se agotó por transacciones sin liberar.</p>
<h2>Solution</h2><p>Se añadió un límite.</p>
<ul><li>Limitar el pool a 20 conexiones.</li><li>Abrir un circuit breaker.</li></ul>
<h2>Verification</h2><p>La latencia volvió a 120 ms durante 24 horas.</p>`

	text, _ := source.HTMLToText(html)
	doc := source.FromText(text, source.Options{})
	draft, err := (extract.MarkedExtractor{}).Extract(context.Background(), doc)
	if err != nil {
		t.Fatalf("Extract(): %v", err)
	}
	if len(draft.Solution.Actions) != 2 {
		t.Fatalf("actions = %d, want 2: %+v", len(draft.Solution.Actions), draft.Solution.Actions)
	}
	if draft.Solution.Summary != "Se añadió un límite." {
		t.Fatalf("summary = %q", draft.Solution.Summary)
	}
}
