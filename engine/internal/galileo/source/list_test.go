package source_test

import (
	"strings"
	"testing"

	"elpanteon/engine/internal/galileo/source"
)

func TestHTMLToTextPreservesListItems(t *testing.T) {
	text, _ := source.HTMLToText(`<h2>Solution</h2><ul><li>Aplicar jitter.</li><li>Serializar por clave.</li></ul>`)
	if !strings.Contains(text, "- Aplicar jitter.") {
		t.Fatalf("no conserva viñetas: %q", text)
	}
	if !strings.Contains(text, "- Serializar por clave.") {
		t.Fatalf("no conserva la segunda viñeta: %q", text)
	}
}
