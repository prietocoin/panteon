package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// SchemaVersion identifica la versión del contrato engineering_case.json.
const SchemaVersion = "1.0"

// Veredictos posibles del filtro Anti-Hype.
const (
	VerdictAccept = "accept"
	VerdictReview = "review"
	VerdictReject = "reject"
)

// EngineeringCase es la salida de Galileo y la entrada de Atlas.
type EngineeringCase struct {
	SchemaVersion string     `json:"schema_version"`
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Source        Source     `json:"source"`
	Symptom       Claim      `json:"symptom"`
	Cause         Claim      `json:"cause"`
	Solution      Solution   `json:"solution"`
	HypeFilter    HypeFilter `json:"hype_filter"`
	Entities      []Entity   `json:"entities,omitempty"`
	GeneratedAt   time.Time  `json:"generated_at"`
}

// Source describe el documento primario del que se extrajo el caso.
type Source struct {
	URL         string    `json:"url,omitempty"`
	Type        string    `json:"type"`
	Title       string    `json:"title,omitempty"`
	RetrievedAt time.Time `json:"retrieved_at"`
	ContentHash string    `json:"content_hash"`
}

// Evidence es una cita literal de la fuente con su localizador.
type Evidence struct {
	Quote   string `json:"quote"`
	Locator string `json:"locator"`
	URL     string `json:"url,omitempty"`
}

// Metric describe una medición con su baseline y método cuando existan.
type Metric struct {
	Name     string  `json:"name"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
	Baseline string  `json:"baseline,omitempty"`
	Period   string  `json:"period,omitempty"`
	Method   string  `json:"method,omitempty"`
}

// Claim representa el síntoma o la causa de un caso de ingeniería.
type Claim struct {
	Summary    string     `json:"summary"`
	Mechanism  string     `json:"mechanism,omitempty"`
	Observable bool       `json:"observable"`
	Metrics    []Metric   `json:"metrics,omitempty"`
	Evidence   []Evidence `json:"evidence"`
}

// Action es una acción concreta de la solución.
type Action struct {
	Description string `json:"description"`
}

// Solution describe la corrección, sus compensaciones y su verificación.
type Solution struct {
	Summary      string     `json:"summary"`
	Actions      []Action   `json:"actions"`
	Tradeoffs    []string   `json:"tradeoffs"`
	Verification string     `json:"verification"`
	Evidence     []Evidence `json:"evidence"`
}

// Flag es una penalización concreta del filtro Anti-Hype.
type Flag struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
	Penalty int    `json:"penalty"`
}

// HypeFilter contiene el score, el veredicto y las razones.
type HypeFilter struct {
	Score             int      `json:"score"`
	Verdict           string   `json:"verdict"`
	Flags             []Flag   `json:"flags"`
	UnsupportedClaims []string `json:"unsupported_claims,omitempty"`
}

// Entity es una entidad técnica mencionada en el caso.
type Entity struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Validate comprueba las invariantes mínimas del contrato.
func (c EngineeringCase) Validate() error {
	var problems []string

	if strings.TrimSpace(c.SchemaVersion) == "" {
		problems = append(problems, "schema_version es obligatorio")
	}
	if strings.TrimSpace(c.ID) == "" {
		problems = append(problems, "id es obligatorio")
	}
	if strings.TrimSpace(c.Title) == "" {
		problems = append(problems, "title es obligatorio")
	}
	if c.GeneratedAt.IsZero() {
		problems = append(problems, "generated_at es obligatorio")
	}
	if strings.TrimSpace(c.Source.Type) == "" {
		problems = append(problems, "source.type es obligatorio")
	}
	if strings.TrimSpace(c.Source.ContentHash) == "" {
		problems = append(problems, "source.content_hash es obligatorio")
	}
	if c.Source.RetrievedAt.IsZero() {
		problems = append(problems, "source.retrieved_at es obligatorio")
	}

	problems = append(problems, claimProblems("symptom", c.Symptom)...)
	problems = append(problems, claimProblems("cause", c.Cause)...)
	problems = append(problems, solutionProblems(c.Solution)...)
	problems = append(problems, hypeProblems(c.HypeFilter)...)

	if len(problems) > 0 {
		return fmt.Errorf("engineering_case inválido: %s", strings.Join(problems, "; "))
	}
	return nil
}

func claimProblems(name string, claim Claim) []string {
	var problems []string
	if strings.TrimSpace(claim.Summary) == "" {
		problems = append(problems, name+".summary es obligatorio")
	}
	if len(claim.Evidence) == 0 {
		problems = append(problems, name+".evidence debe tener al menos una cita")
	}
	for i, evidence := range claim.Evidence {
		if strings.TrimSpace(evidence.Quote) == "" {
			problems = append(problems, fmt.Sprintf("%s.evidence[%d].quote es obligatorio", name, i))
		}
		if strings.TrimSpace(evidence.Locator) == "" {
			problems = append(problems, fmt.Sprintf("%s.evidence[%d].locator es obligatorio", name, i))
		}
	}
	for i, metric := range claim.Metrics {
		if strings.TrimSpace(metric.Name) == "" {
			problems = append(problems, fmt.Sprintf("%s.metrics[%d].name es obligatorio", name, i))
		}
		if strings.TrimSpace(metric.Unit) == "" {
			problems = append(problems, fmt.Sprintf("%s.metrics[%d].unit es obligatorio", name, i))
		}
	}
	return problems
}

func solutionProblems(solution Solution) []string {
	var problems []string
	if strings.TrimSpace(solution.Summary) == "" {
		problems = append(problems, "solution.summary es obligatorio")
	}
	if len(solution.Evidence) == 0 {
		problems = append(problems, "solution.evidence debe tener al menos una cita")
	}
	for i, action := range solution.Actions {
		if strings.TrimSpace(action.Description) == "" {
			problems = append(problems, fmt.Sprintf("solution.actions[%d].description es obligatorio", i))
		}
	}
	for i, evidence := range solution.Evidence {
		if strings.TrimSpace(evidence.Quote) == "" {
			problems = append(problems, fmt.Sprintf("solution.evidence[%d].quote es obligatorio", i))
		}
		if strings.TrimSpace(evidence.Locator) == "" {
			problems = append(problems, fmt.Sprintf("solution.evidence[%d].locator es obligatorio", i))
		}
	}
	return problems
}

func hypeProblems(hype HypeFilter) []string {
	var problems []string
	if hype.Score < 0 || hype.Score > 100 {
		problems = append(problems, "hype_filter.score debe estar entre 0 y 100")
	}
	switch hype.Verdict {
	case VerdictAccept, VerdictReview, VerdictReject:
	default:
		problems = append(problems, "hype_filter.verdict debe ser accept, review o reject")
	}
	return problems
}

// ContentHash normaliza el texto y devuelve su SHA-256 con prefijo.
func ContentHash(text string) string {
	normalized := strings.Join(strings.Fields(text), " ")
	sum := sha256.Sum256([]byte(normalized))
	return "sha256:" + hex.EncodeToString(sum[:])
}
