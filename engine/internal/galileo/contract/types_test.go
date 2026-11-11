package contract_test

import (
	"strings"
	"testing"
	"time"

	"elpanteon/engine/internal/galileo/contract"
)

func validCase() contract.EngineeringCase {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	evidence := []contract.Evidence{{
		Quote:   "La API devolvió errores 503.",
		Locator: "line:4",
		URL:     "https://example.com/postmortem",
	}}
	return contract.EngineeringCase{
		SchemaVersion: contract.SchemaVersion,
		ID:            "sha256:abc",
		Title:         "Incident: connection pool",
		Source: contract.Source{
			URL:         "https://example.com/postmortem",
			Type:        "postmortem",
			RetrievedAt: now,
			ContentHash: "sha256:abc",
		},
		Symptom: contract.Claim{
			Summary:    "Errores 503 durante 12 minutos.",
			Observable: true,
			Evidence:   evidence,
		},
		Cause: contract.Claim{
			Summary:   "El pool de conexiones se agotó.",
			Mechanism: "Una migración no liberó las conexiones.",
			Evidence:  evidence,
		},
		Solution: contract.Solution{
			Summary:      "Se limitó el pool.",
			Actions:      []contract.Action{{Description: "Limitar el pool a 20 conexiones."}},
			Tradeoffs:    []string{"Menor throughput pico."},
			Verification: "La latencia volvió a 180 ms.",
			Evidence:     evidence,
		},
		HypeFilter: contract.HypeFilter{
			Score:   100,
			Verdict: contract.VerdictAccept,
			Flags:   []contract.Flag{},
		},
		GeneratedAt: now,
	}
}

func TestValidateAcceptsCompleteCase(t *testing.T) {
	if err := validCase().Validate(); err != nil {
		t.Fatalf("Validate() devolvió error: %v", err)
	}
}

func TestValidateRejectsMissingEvidence(t *testing.T) {
	engineeringCase := validCase()
	engineeringCase.Symptom.Evidence = nil
	engineeringCase.Cause.Evidence = []contract.Evidence{}
	engineeringCase.Solution.Evidence = nil

	err := engineeringCase.Validate()
	if err == nil {
		t.Fatal("Validate() debía fallar sin evidencia")
	}
	if !strings.Contains(err.Error(), "symptom.evidence") {
		t.Fatalf("error inesperado: %v", err)
	}
}

func TestValidateRejectsInvalidVerdict(t *testing.T) {
	engineeringCase := validCase()
	engineeringCase.HypeFilter.Verdict = "maybe"

	err := engineeringCase.Validate()
	if err == nil {
		t.Fatal("Validate() debía fallar con un veredicto inválido")
	}
	if !strings.Contains(err.Error(), "verdict") {
		t.Fatalf("error inesperado: %v", err)
	}
}

func TestContentHashIgnoresWhitespace(t *testing.T) {
	first := contract.ContentHash("hola   mundo\n")
	second := contract.ContentHash("  hola mundo ")
	if first != second {
		t.Fatalf("hashes distintos: %q != %q", first, second)
	}
	if !strings.HasPrefix(first, "sha256:") {
		t.Fatalf("hash sin prefijo: %q", first)
	}
}
