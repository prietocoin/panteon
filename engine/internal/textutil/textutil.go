package textutil

import (
	"regexp"
	"strings"
)

var whitespaceRe = regexp.MustCompile(`\s+`)

// Sentences divide un texto en oraciones sin romper decimales.
func Sentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	var out []string
	start := 0
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '.', '!', '?':
			next := i + 1
			for next < len(runes) && (runes[next] == ' ' || runes[next] == '\n' || runes[next] == '\t') {
				next++
			}
			if next == len(runes) || next > i+1 {
				sentence := strings.TrimSpace(string(runes[start : i+1]))
				if sentence != "" {
					out = append(out, sentence)
				}
				start = next
				i = next - 1
			}
		}
	}
	if start < len(runes) {
		sentence := strings.TrimSpace(string(runes[start:]))
		if sentence != "" {
			out = append(out, sentence)
		}
	}
	return out
}

// FirstSentence devuelve la primera oración o el texto completo.
func FirstSentence(text string) string {
	sentences := Sentences(text)
	if len(sentences) == 0 {
		return strings.TrimSpace(text)
	}
	return sentences[0]
}

// Truncate recorta por cantidad de runas y agrega elipsis.
func Truncate(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	if maxRunes <= 1 {
		return string(runes[:maxRunes])
	}
	return strings.TrimSpace(string(runes[:maxRunes-1])) + "…"
}

// Clean colapsa espacios y saltos de línea.
func Clean(text string) string {
	return strings.TrimSpace(whitespaceRe.ReplaceAllString(text, " "))
}

// Bullets convierte un texto en viñetas cortas.
func Bullets(text string, max int) []string {
	sentences := Sentences(text)
	if len(sentences) == 0 {
		sentences = []string{text}
	}
	out := make([]string, 0, max)
	for _, sentence := range sentences {
		sentence = Truncate(Clean(sentence), 180)
		if sentence == "" {
			continue
		}
		out = append(out, sentence)
		if len(out) == max {
			break
		}
	}
	return out
}
