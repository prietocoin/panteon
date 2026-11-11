package filter

import (
	"fmt"
	"regexp"
	"strings"

	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/galileo/extract"
)

// Penalizaciones y umbrales del filtro Anti-Hype.
const (
	penaltyHypePerHit      = 10
	penaltyHypeMax         = 40
	penaltyMissingEvidence = 35
	penaltyVagueCause      = 35
	penaltyUnsupportedNum  = 6
	penaltyNumericMax      = 18
	penaltyNoTradeoffs     = 10
	penaltyNoVerification  = 15
	penaltyNotObservable   = 20

	acceptThreshold = 70
	reviewThreshold = 40
)

var accentFolder = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	"Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ü", "u", "Ñ", "n",
)

// Términos promocionales que no constituyen evidencia técnica.
var hypeTerms = []string{
	"revolucionario", "revolucionaria", "revolucion",
	"cambio de juego", "game changer", "game-changing",
	"disruptivo", "disruptiva", "disrupt",
	"10x", "10 veces", "100x",
	"de clase mundial", "world-class", "best-in-class",
	"de ultima generacion", "next-generation", "cutting-edge",
	"sin esfuerzo", "seamless", "effortless",
	"magico", "magica", "silver bullet", "bala de plata",
	"blazing fast", "ultrarrrapido", "ultra-rapido",
}

var vagueCauseTerms = []string{
	"error humano", "human error",
	"mala configuracion", "bad configuration", "misconfiguration", "configuracion incorrecta",
	"fallo del sistema", "system failure",
	"problema de red", "network issue",
	"codigo malo", "bad code",
	"falta de recursos", "not enough resources",
}

var numberRe = regexp.MustCompile(`\d+(?:[.,]\d+)?`)

// Evaluate aplica las reglas Anti-Hype y devuelve score, veredicto y flags.
func Evaluate(draft extract.Draft) contract.HypeFilter {
	flags := make([]contract.Flag, 0)
	unsupported := make([]string, 0)
	score := 100

	deduct := func(rule, message string, penalty int) {
		flags = append(flags, contract.Flag{Rule: rule, Message: message, Penalty: penalty})
		score -= penalty
	}

	allText := fold(strings.Join([]string{
		draft.Symptom.Summary,
		draft.Symptom.Mechanism,
		draft.Cause.Summary,
		draft.Cause.Mechanism,
		draft.Solution.Summary,
	}, " "))

	matchedHype := make([]string, 0)
	for _, term := range hypeTerms {
		if strings.Contains(allText, term) {
			matchedHype = append(matchedHype, term)
		}
	}
	if len(matchedHype) > 0 {
		penalty := len(matchedHype) * penaltyHypePerHit
		if penalty > penaltyHypeMax {
			penalty = penaltyHypeMax
		}
		deduct(
			"hype_lexicon",
			"Léxico promocional sin evidencia: "+strings.Join(matchedHype, ", "),
			penalty,
		)
		for _, term := range matchedHype {
			unsupported = append(unsupported, fmt.Sprintf("%q (léxico promocional)", term))
		}
	}

	if len(draft.Symptom.Evidence) == 0 {
		deduct("missing_evidence", "El síntoma no tiene ninguna cita de la fuente.", penaltyMissingEvidence)
	}
	if len(draft.Cause.Evidence) == 0 {
		deduct("missing_evidence", "La causa no tiene ninguna cita de la fuente.", penaltyMissingEvidence)
	}
	if len(draft.Solution.Evidence) == 0 {
		deduct("missing_evidence", "La solución no tiene ninguna cita de la fuente.", penaltyMissingEvidence)
	}

	causeText := fold(strings.Join([]string{draft.Cause.Summary, draft.Cause.Mechanism}, " "))
	for _, term := range vagueCauseTerms {
		if strings.Contains(causeText, term) {
			deduct(
				"vague_cause",
				fmt.Sprintf("La causa usa una explicación vaga (%q) en lugar de un mecanismo técnico.", term),
				penaltyVagueCause,
			)
			unsupported = append(unsupported, fmt.Sprintf("causa vaga: %q", term))
			break
		}
	}

	quotes := fold(evidenceText(draft))
	missingNumbers := make([]string, 0)
	seenNumbers := make(map[string]bool)
	for _, summary := range []string{
		draft.Symptom.Summary,
		draft.Cause.Summary,
		draft.Solution.Summary,
	} {
		for _, rawNumber := range numberRe.FindAllString(summary, -1) {
			number := strings.ReplaceAll(rawNumber, ",", ".")
			if seenNumbers[number] {
				continue
			}
			seenNumbers[number] = true
			if !strings.Contains(quotes, number) {
				missingNumbers = append(missingNumbers, number)
			}
		}
	}
	if len(missingNumbers) > 0 {
		penalty := len(missingNumbers) * penaltyUnsupportedNum
		if penalty > penaltyNumericMax {
			penalty = penaltyNumericMax
		}
		deduct(
			"numeric_claim_without_evidence",
			"Datos numéricos sin cita literal: "+strings.Join(missingNumbers, ", "),
			penalty,
		)
		for _, number := range missingNumbers {
			unsupported = append(unsupported, fmt.Sprintf("%q (dato sin cita)", number))
		}
	}

	if !draft.Symptom.Observable {
		deduct(
			"not_observable",
			"El síntoma no describe una señal observable (métrica, error o caída).",
			penaltyNotObservable,
		)
	}
	if len(draft.Solution.Tradeoffs) == 0 {
		deduct(
			"missing_tradeoffs",
			"La solución no declara compensaciones o riesgos.",
			penaltyNoTradeoffs,
		)
	}
	if strings.TrimSpace(draft.Solution.Verification) == "" {
		deduct(
			"missing_verification",
			"La solución no explica cómo se verificó la corrección.",
			penaltyNoVerification,
		)
	}

	if score < 0 {
		score = 0
	}

	verdict := contract.VerdictAccept
	switch {
	case score >= acceptThreshold:
		verdict = contract.VerdictAccept
	case score >= reviewThreshold:
		verdict = contract.VerdictReview
	default:
		verdict = contract.VerdictReject
	}

	return contract.HypeFilter{
		Score:             score,
		Verdict:           verdict,
		Flags:             flags,
		UnsupportedClaims: unsupported,
	}
}

func evidenceText(draft extract.Draft) string {
	parts := make([]string, 0)
	for _, group := range [][]contract.Evidence{
		draft.Symptom.Evidence,
		draft.Cause.Evidence,
		draft.Solution.Evidence,
	} {
		for _, evidence := range group {
			parts = append(parts, evidence.Quote)
		}
	}
	return strings.Join(parts, " ")
}

func fold(value string) string {
	return accentFolder.Replace(strings.ToLower(strings.TrimSpace(value)))
}
