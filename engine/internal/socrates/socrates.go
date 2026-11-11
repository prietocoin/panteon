package socrates

import (
	"strings"

	"elpanteon/engine/internal/galileo/contract"
)

// Review valida el rigor técnico de un caso antes de gastar recursos de render.
func Review(engineeringCase contract.EngineeringCase) contract.RigorReport {
	report := contract.RigorReport{
		Passed:   true,
		Score:    100,
		Findings: make([]contract.RigorFinding, 0),
	}

	penalize := func(rule, message string, penalty int) {
		report.Findings = append(report.Findings, contract.RigorFinding{Rule: rule, Message: message})
		report.Score -= penalty
	}

	if engineeringCase.HypeFilter.Verdict == contract.VerdictReject {
		penalize("case_rejected", "Galileo rechazó el caso por falta de evidencia o exceso de hype.", 40)
	}
	if engineeringCase.HypeFilter.Verdict == contract.VerdictReview {
		penalize("case_in_review", "Galileo marcó el caso para revisión humana.", 15)
	}
	if strings.TrimSpace(engineeringCase.Cause.Mechanism) == "" {
		penalize("missing_mechanism", "La causa no describe un mecanismo técnico.", 20)
	}
	if len(engineeringCase.Solution.Actions) == 0 {
		penalize("missing_actions", "La solución no contiene acciones concretas.", 15)
	}
	if strings.TrimSpace(engineeringCase.Solution.Verification) == "" {
		penalize("missing_verification", "No se declara cómo se verificó la solución.", 10)
	}
	if len(engineeringCase.Solution.Tradeoffs) == 0 {
		penalize("missing_tradeoffs", "No se declaran compensaciones o riesgos.", 10)
	}
	for _, metric := range append(engineeringCase.Symptom.Metrics, engineeringCase.Cause.Metrics...) {
		if strings.TrimSpace(metric.Baseline) == "" || strings.TrimSpace(metric.Method) == "" {
			penalize("metric_without_context", "Hay métricas sin baseline o método de medición.", 10)
			break
		}
	}

	if report.Score < 0 {
		report.Score = 0
	}
	report.Passed = report.Score >= 70 && engineeringCase.HypeFilter.Verdict != contract.VerdictReject
	return report
}
