package source_test

import (
	"context"
	"strings"
	"testing"

	"elpanteon/engine/internal/galileo/source"
)

func TestNormalizeTextCollapsesBlankLines(t *testing.T) {
	got := source.NormalizeText("a\r\n\r\n\r\nb  \r\n")
	want := "a\n\nb"
	if got != want {
		t.Fatalf("NormalizeText() = %q, want %q", got, want)
	}
}

func TestHTMLToTextExtractsTitleAndDropsScripts(t *testing.T) {
	raw := `<html><head><title>Postmortem de pagos</title>
	<style>body { color: red; }</style></head>
	<body><h1>Symptom</h1><p>503 errors for 12 minutes</p>
	<script>alert("x")</script></body></html>`

	text, title := source.HTMLToText(raw)
	if title != "Postmortem de pagos" {
		t.Fatalf("title = %q", title)
	}
	if !strings.Contains(text, "503 errors") {
		t.Fatalf("texto sin contenido: %q", text)
	}
	if strings.Contains(text, "color: red") || strings.Contains(text, "alert") {
		t.Fatalf("texto contiene script o estilo: %q", text)
	}
}

func TestValidatePublicURLBlocksPrivateDestinations(t *testing.T) {
	ctx := context.Background()
	for _, target := range []string{
		"http://127.0.0.1/admin",
		"http://localhost:8080/",
		"http://[::1]/",
		"file:///etc/passwd",
	} {
		if err := source.ValidatePublicURL(ctx, target); err == nil {
			t.Errorf("ValidatePublicURL(%q) debía bloquear", target)
		}
	}
}

func TestFromTextComputesHashAndTitle(t *testing.T) {
	doc := source.FromText("# Incident: pool\n\n## Symptom\n503 errors", source.Options{})
	if doc.Title != "Incident: pool" {
		t.Fatalf("title = %q", doc.Title)
	}
	if doc.Type != "other" {
		t.Fatalf("type = %q", doc.Type)
	}
	if !strings.HasPrefix(doc.ContentHash, "sha256:") {
		t.Fatalf("content_hash = %q", doc.ContentHash)
	}
	if doc.RetrievedAt.IsZero() {
		t.Fatal("RetrievedAt no debe ser cero")
	}
}
