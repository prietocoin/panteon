package extract

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/galileo/source"
)

// ErrMissingSections indica que la fuente no contiene la Tríada completa.
var ErrMissingSections = errors.New("faltan secciones obligatorias")

// Draft es la extracción previa al filtro Anti-Hype.
type Draft struct {
	Title      string
	Symptom    contract.Claim
	Cause      contract.Claim
	Solution   contract.Solution
	Entities   []contract.Entity
	SourceType string
}

// Extractor convierte una fuente en un borrador de caso de ingeniería.
type Extractor interface {
	Extract(ctx context.Context, doc source.Document) (Draft, error)
}

// MarkedExtractor es el extractor determinista del MVP.
// Lee secciones con encabezados conocidos (Symptom, Cause, Solution, ...).
type MarkedExtractor struct{}

type lineRef struct {
	text string
	line int
}

var (
	numberingRe = regexp.MustCompile(`^\d+[.)]\s*`)
	bulletRe    = regexp.MustCompile(`^\s*(?:[-*•]|\d+[.)])\s+`)
	digitRe     = regexp.MustCompile(`\d`)
)

var accentFolder = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	"Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ü", "u", "Ñ", "n",
)

// Extract implementa Extractor.
func (MarkedExtractor) Extract(ctx context.Context, doc source.Document) (Draft, error) {
	if err := ctx.Err(); err != nil {
		return Draft{}, err
	}

	sections := map[string][]lineRef{}
	title := ""
	current := ""

	for i, raw := range strings.Split(doc.Text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		kind, inline := parseHeading(line)
		if kind == "title" {
			if inline != "" {
				title = inline
			}
			current = ""
			continue
		}
		if kind != "" {
			current = kind
			if inline != "" {
				sections[current] = append(sections[current], lineRef{text: inline, line: i + 1})
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			current = ""
			continue
		}
		if current == "" {
			continue
		}
		sections[current] = append(sections[current], lineRef{text: line, line: i + 1})
	}

	var missing []string
	for _, required := range []string{"symptom", "cause", "solution"} {
		if len(sections[required]) == 0 {
			missing = append(missing, required)
		}
	}
	if len(missing) > 0 {
		return Draft{}, fmt.Errorf("%w: %s", ErrMissingSections, strings.Join(missing, ", "))
	}

	symptomRefs := sections["symptom"]
	causeRefs := sections["cause"]
	solutionRefs := sections["solution"]
	tradeoffRefs := sections["tradeoffs"]
	verificationRefs := sections["verification"]

	symptomSummary := joinRefs(symptomRefs)
	causeSummary := joinRefs(causeRefs)

	solutionSummaryRefs := nonBulletRefs(solutionRefs)
	if len(solutionSummaryRefs) == 0 {
		solutionSummaryRefs = solutionRefs
	}
	solutionSummary := joinRefs(solutionSummaryRefs)

	actions := bullets(solutionRefs)
	if len(actions) == 0 {
		actions = []string{solutionSummary}
	}
	tradeoffs := textLines(tradeoffRefs)

	draft := Draft{
		Title: firstNonEmpty(title, doc.Title),
		Symptom: contract.Claim{
			Summary:    symptomSummary,
			Observable: isObservable(symptomSummary),
			Evidence:   refsToEvidence(symptomRefs, doc.URL),
		},
		Cause: contract.Claim{
			Summary:   causeSummary,
			Mechanism: causeSummary,
			Evidence:  refsToEvidence(causeRefs, doc.URL),
		},
		Solution: contract.Solution{
			Summary:      solutionSummary,
			Actions:      make([]contract.Action, 0, len(actions)),
			Tradeoffs:    tradeoffs,
			Verification: joinRefs(verificationRefs),
			Evidence:     refsToEvidence(solutionRefs, doc.URL),
		},
		SourceType: doc.Type,
	}
	for _, action := range actions {
		draft.Solution.Actions = append(draft.Solution.Actions, contract.Action{Description: action})
	}
	return draft, nil
}

func parseHeading(raw string) (kind, inline string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ""
	}

	trimmed = strings.TrimSpace(strings.Trim(trimmed, "*"))
	trimmed = strings.TrimLeft(trimmed, "#")
	trimmed = strings.TrimSpace(trimmed)
	trimmed = numberingRe.ReplaceAllString(trimmed, "")
	trimmed = strings.TrimSpace(strings.Trim(trimmed, "*_` "))

	if trimmed == "" {
		return "", ""
	}

	key := trimmed
	if index := strings.Index(trimmed, ":"); index >= 0 {
		key = strings.TrimSpace(trimmed[:index])
		inline = strings.TrimSpace(trimmed[index+1:])
	}

	kind = classifyHeading(fold(key))
	if kind == "" {
		return "", ""
	}
	return kind, inline
}

func classifyHeading(key string) string {
	key = strings.TrimSpace(key)
	key = strings.TrimRight(key, "?:!.")
	key = strings.Trim(key, " ._-")

	switch key {
	case "symptom", "symptoms", "sintoma", "sintomas", "senal", "senales",
		"what happened", "impact", "customer impact", "how did the outage impact customers",
		"incident summary", "summary", "overview", "the incident", "what was the impact":
		return "symptom"
	case "cause", "root cause", "causa", "causa raiz", "root-cause",
		"how did the incident occur", "why did it happen", "what caused", "what caused it",
		"technical analysis", "analysis", "postmortem analysis", "why it happened":
		return "cause"
	case "solution", "solutions", "solucion", "soluciones", "fix", "fixes",
		"remediation", "remediation and follow-up steps", "resolution", "resolucion",
		"mitigation", "what we did", "actions", "acciones", "action items",
		"corrective actions", "follow-up", "follow-up steps":
		return "solution"
	case "tradeoff", "tradeoffs", "trade-off", "trade-offs", "trade offs", "compensacion",
		"compensaciones", "cost", "costs", "coste", "costes", "riesgo", "riesgos", "risks":
		return "tradeoffs"
	case "verification", "verificacion", "validation", "validacion",
		"como se verifico", "how we verified", "evidence of fix",
		"monitoring", "results", "result":
		return "verification"
	case "title", "titulo":
		return "title"
	default:
		return ""
	}
}

func joinRefs(refs []lineRef) string {
	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		parts = append(parts, ref.text)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func nonBulletRefs(refs []lineRef) []lineRef {
	out := make([]lineRef, 0, len(refs))
	for _, ref := range refs {
		if _, isBullet := stripBullet(ref.text); !isBullet {
			out = append(out, ref)
		}
	}
	return out
}

func bullets(refs []lineRef) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		if text, ok := stripBullet(ref.text); ok {
			out = append(out, text)
		}
	}
	return out
}

func textLines(refs []lineRef) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		if text, ok := stripBullet(ref.text); ok {
			out = append(out, text)
		} else {
			out = append(out, ref.text)
		}
	}
	return out
}

func stripBullet(line string) (string, bool) {
	if !bulletRe.MatchString(line) {
		return "", false
	}
	return strings.TrimSpace(bulletRe.ReplaceAllString(line, "")), true
}

func refsToEvidence(refs []lineRef, url string) []contract.Evidence {
	const maxEvidence = 12
	if len(refs) > maxEvidence {
		refs = refs[:maxEvidence]
	}
	out := make([]contract.Evidence, 0, len(refs))
	for _, ref := range refs {
		out = append(out, contract.Evidence{
			Quote:   ref.text,
			Locator: fmt.Sprintf("line:%d", ref.line),
			URL:     url,
		})
	}
	return out
}

func isObservable(text string) bool {
	if digitRe.MatchString(text) {
		return true
	}
	folded := fold(text)
	for _, keyword := range []string{
		"latencia", "error", "caida", "timeout", "cpu", "memoria",
		"throughput", "disponibilidad", "traza", "stack trace", "excepcion",
	} {
		if strings.Contains(folded, keyword) {
			return true
		}
	}
	return false
}

func fold(value string) string {
	return accentFolder.Replace(strings.ToLower(strings.TrimSpace(value)))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
