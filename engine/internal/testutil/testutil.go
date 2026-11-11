package testutil

import (
	"time"

	"elpanteon/engine/internal/galileo/contract"
)

// Case devuelve un engineering_case válido y aceptado para tests de fases.
func Case() contract.EngineeringCase {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	evidence := []contract.Evidence{
		{Quote: "La API devolvió errores 503 durante 12 minutos.", Locator: "line:5", URL: "https://example.com/postmortem"},
		{Quote: "El pool de conexiones se agotó porque una migración no liberó conexiones.", Locator: "line:10", URL: "https://example.com/postmortem"},
	}
	return contract.EngineeringCase{
		SchemaVersion: contract.SchemaVersion,
		ID:            contract.ContentHash("fixture"),
		Title:         "Incident: connection pool exhaustion",
		Source: contract.Source{
			URL:         "https://example.com/postmortem",
			Type:        "postmortem",
			Title:       "Incident: connection pool exhaustion",
			RetrievedAt: now,
			ContentHash: contract.ContentHash("fixture"),
		},
		Symptom: contract.Claim{
			Summary:    "La API devolvió errores 503 durante 12 minutos.",
			Observable: true,
			Evidence:   evidence[:1],
		},
		Cause: contract.Claim{
			Summary:   "El pool de conexiones se agotó porque una migración no liberó conexiones.",
			Mechanism: "Una migración abrió transacciones sin liberar conexiones.",
			Evidence:  evidence[1:],
		},
		Solution: contract.Solution{
			Summary: "Se añadió un límite de conexiones y un circuit breaker.",
			Actions: []contract.Action{
				{Description: "Limitar el pool a 20 conexiones por instancia."},
				{Description: "Abrir el circuit breaker tras 5 timeouts."},
			},
			Tradeoffs:    []string{"Menor throughput pico durante ráfagas."},
			Verification: "La latencia p99 volvió a 180 ms durante 72 horas.",
			Evidence:     evidence[1:],
		},
		HypeFilter: contract.HypeFilter{
			Score:   100,
			Verdict: contract.VerdictAccept,
			Flags:   []contract.Flag{},
		},
		GeneratedAt: now,
	}
}
