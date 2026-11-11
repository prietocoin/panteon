package filter_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/galileo/extract"
	"elpanteon/engine/internal/galileo/filter"
	"elpanteon/engine/internal/galileo/source"
)

func loadDraft(t *testing.T) extract.Draft {
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
	return draft
}

func TestEvaluateAcceptsCleanCase(t *testing.T) {
	result := filter.Evaluate(loadDraft(t))
	if result.Verdict != contract.VerdictAccept {
		t.Fatalf("verdict = %s, flags = %+v", result.Verdict, result.Flags)
	}
	if result.Score != 100 {
		t.Fatalf("score = %d, flags = %+v", result.Score, result.Flags)
	}
}

func TestEvaluatePenalizesHypeLexicon(t *testing.T) {
	draft := loadDraft(t)
	draft.Symptom.Summary += " Una arquitectura revolucionaria, disruptiva, de clase mundial, sin esfuerzo y 10x."

	result := filter.Evaluate(draft)
	if result.Score >= 70 {
		t.Fatalf("score = %d, se esperaba penalización", result.Score)
	}
	if result.Verdict != contract.VerdictReview {
		t.Fatalf("verdict = %s, want review", result.Verdict)
	}
	if !hasRule(result, "hype_lexicon") {
		t.Fatalf("falta flag hype_lexicon: %+v", result.Flags)
	}
	if len(result.UnsupportedClaims) == 0 {
		t.Fatal("debía registrar claims no soportados")
	}
}

func TestEvaluatePenalizesMissingEvidence(t *testing.T) {
	draft := loadDraft(t)
	draft.Symptom.Evidence = nil

	result := filter.Evaluate(draft)
	if result.Verdict == contract.VerdictAccept {
		t.Fatalf("verdict = %s, no debía aceptar sin evidencia", result.Verdict)
	}
	if !hasRule(result, "missing_evidence") {
		t.Fatalf("falta flag missing_evidence: %+v", result.Flags)
	}
}

func TestEvaluatePenalizesVagueCause(t *testing.T) {
	draft := loadDraft(t)
	draft.Cause.Summary = "Fue un error humano."
	draft.Cause.Mechanism = "Fue un error humano."

	result := filter.Evaluate(draft)
	if result.Verdict == contract.VerdictAccept {
		t.Fatalf("verdict = %s, no debía aceptar una causa vaga", result.Verdict)
	}
	if !hasRule(result, "vague_cause") {
		t.Fatalf("falta flag vague_cause: %+v", result.Flags)
	}
}

func TestEvaluateFlagsNumbersWithoutQuote(t *testing.T) {
	draft := loadDraft(t)
	draft.Symptom.Summary = "La latencia llegó a 9999 ms."

	result := filter.Evaluate(draft)
	if !hasRule(result, "numeric_claim_without_evidence") {
		t.Fatalf("falta flag numérico: %+v", result.Flags)
	}
	if !strings.Contains(strings.Join(result.UnsupportedClaims, " "), "9999") {
		t.Fatalf("claims = %+v", result.UnsupportedClaims)
	}
}

func TestEvaluateRejectsCaseWithoutEvidenceOrMechanism(t *testing.T) {
	draft := extract.Draft{
		Symptom: contract.Claim{Summary: "Revolucionario y mágico."},
		Cause:   contract.Claim{Summary: "Error humano."},
		Solution: contract.Solution{
			Summary: "Sin esfuerzo.",
		},
	}

	result := filter.Evaluate(draft)
	if result.Verdict != contract.VerdictReject {
		t.Fatalf("verdict = %s, want reject (score %d)", result.Verdict, result.Score)
	}
}

func hasRule(result contract.HypeFilter, rule string) bool {
	for _, item := range result.Flags {
		if item.Rule == rule {
			return true
		}
	}
	return false
}
