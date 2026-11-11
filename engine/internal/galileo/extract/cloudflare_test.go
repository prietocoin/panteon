package extract_test

import (
	"context"
	"testing"

	"elpanteon/engine/internal/galileo/extract"
	"elpanteon/engine/internal/galileo/source"
)

func TestMarkedExtractorRecognizesCloudflareHeadings(t *testing.T) {
	html := `<h2>How did the outage impact customers?</h2><p>La API devolvió errores 503 durante 12 minutos.</p>
<h2>How did the incident occur?</h2><p>El pool se agotó porque una migración no liberó conexiones.</p>
<h2>Remediation and follow-up steps</h2><p>Se añadió un límite de conexiones y un circuit breaker.</p>`

	text, _ := source.HTMLToText(html)
	doc := source.FromText(text, source.Options{})
	draft, err := (extract.MarkedExtractor{}).Extract(context.Background(), doc)
	if err != nil {
		t.Fatalf("Extract(): %v", err)
	}
	if draft.Symptom.Summary == "" || draft.Cause.Summary == "" || draft.Solution.Summary == "" {
		t.Fatalf("tríada incompleta: %+v", draft)
	}
}
