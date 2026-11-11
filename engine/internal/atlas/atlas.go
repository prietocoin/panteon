package atlas

import (
	"context"
	"fmt"
	"strings"
	"time"

	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/textutil"
)

// Builder transforma un engineering_case en un dossier técnico.
type Builder interface {
	Build(ctx context.Context, engineeringCase contract.EngineeringCase) (contract.Dossier, error)
}

// DeterministicBuilder genera el dossier a partir del caso, sin red.
// Es el modo offline del MVP; un builder de GitHub se conecta por la misma interfaz.
type DeterministicBuilder struct {
	Now func() time.Time
}

// Build implementa Builder.
func (b DeterministicBuilder) Build(ctx context.Context, engineeringCase contract.EngineeringCase) (contract.Dossier, error) {
	if err := ctx.Err(); err != nil {
		return contract.Dossier{}, err
	}
	if err := engineeringCase.Validate(); err != nil {
		return contract.Dossier{}, fmt.Errorf("atlas: %w", err)
	}

	now := time.Now
	if b.Now != nil {
		now = b.Now
	}

	source := engineeringCase.Source.URL
	if source == "" {
		source = engineeringCase.Source.ContentHash
	}

	symptomEvidence := quotes(engineeringCase.Symptom.Evidence)
	causeMechanism := strings.TrimSpace(engineeringCase.Cause.Mechanism)
	if causeMechanism == "" {
		causeMechanism = engineeringCase.Cause.Summary
	}
	verification := strings.TrimSpace(engineeringCase.Solution.Verification)
	if verification == "" {
		verification = "No declarada en la fuente."
	}
	tradeoffs := strings.Join(engineeringCase.Solution.Tradeoffs, " ")
	if tradeoffs == "" {
		tradeoffs = "No declaradas en la fuente."
	}

	dossier := contract.Dossier{
		SchemaVersion: contract.DossierSchemaVersion,
		CaseID:        engineeringCase.ID,
		Title:         engineeringCase.Title,
		Summary: fmt.Sprintf(
			"Síntoma: %s Causa: %s Solución: %s",
			textutil.Clean(engineeringCase.Symptom.Summary),
			textutil.Clean(engineeringCase.Cause.Summary),
			textutil.Clean(engineeringCase.Solution.Summary),
		),
		Facts: []contract.Fact{
			{Label: "Síntoma", Value: engineeringCase.Symptom.Summary, Source: firstLocator(engineeringCase.Symptom.Evidence)},
			{Label: "Causa", Value: engineeringCase.Cause.Summary, Source: firstLocator(engineeringCase.Cause.Evidence)},
			{Label: "Solución", Value: engineeringCase.Solution.Summary, Source: firstLocator(engineeringCase.Solution.Evidence)},
			{Label: "Verificación", Value: verification, Source: source},
			{Label: "Trade-offs", Value: tradeoffs, Source: source},
		},
		Artifacts: []contract.Artifact{
			{
				ID:      "symptom-evidence",
				Kind:    "benchmark",
				Title:   "Evidencia del síntoma",
				Content: symptomEvidence,
				Source:  source,
			},
			{
				ID:      "cause-mechanism",
				Kind:    "diagram",
				Title:   "Cadena causal",
				Content: causeMechanism,
				Source:  source,
			},
			{
				ID:      "solution-code",
				Kind:    "code",
				Title:   "Plan de remediación",
				Content: remediationCode(engineeringCase),
				Source:  source,
			},
			{
				ID:      "verification",
				Kind:    "reference",
				Title:   "Verificación y compensaciones",
				Content: verification + "\n\nTrade-offs: " + tradeoffs,
				Source:  source,
			},
		},
		GeneratedAt: now().UTC(),
	}

	if err := dossier.Validate(); err != nil {
		return contract.Dossier{}, fmt.Errorf("atlas: %w", err)
	}
	return dossier, nil
}

func remediationCode(engineeringCase contract.EngineeringCase) string {
	var builder strings.Builder
	builder.WriteString("# Plan de remediación\n")
	builder.WriteString("case: ")
	builder.WriteString(engineeringCase.ID)
	builder.WriteString("\nremediation:\n")
	if len(engineeringCase.Solution.Actions) == 0 {
		builder.WriteString("  - Revisar la solución en la fuente primaria.\n")
	}
	for _, action := range engineeringCase.Solution.Actions {
		builder.WriteString("  - ")
		builder.WriteString(textutil.Clean(action.Description))
		builder.WriteString("\n")
	}
	builder.WriteString("verification: ")
	builder.WriteString(textutil.FirstSentence(engineeringCase.Solution.Verification))
	builder.WriteString("\n")
	return strings.TrimSpace(builder.String())
}

func quotes(evidence []contract.Evidence) string {
	parts := make([]string, 0, len(evidence))
	for _, item := range evidence {
		parts = append(parts, item.Quote)
	}
	if len(parts) == 0 {
		return "Sin evidencia citada."
	}
	return strings.Join(parts, "\n")
}

func firstLocator(evidence []contract.Evidence) string {
	if len(evidence) == 0 {
		return "sin localizador"
	}
	return evidence[0].Locator
}
