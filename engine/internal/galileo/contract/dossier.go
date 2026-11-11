package contract

import (
	"fmt"
	"strings"
	"time"
)

// DossierSchemaVersion identifica la versión de dossier_tecnico.json.
const DossierSchemaVersion = "1.0"

// Dossier es la salida de Atlas y la entrada de Cervantes.
type Dossier struct {
	SchemaVersion string     `json:"schema_version"`
	CaseID        string     `json:"case_id"`
	Title         string     `json:"title"`
	Summary       string     `json:"summary"`
	Facts         []Fact     `json:"facts"`
	Artifacts     []Artifact `json:"artifacts"`
	GeneratedAt   time.Time  `json:"generated_at"`
}

// Fact es un dato verificado del caso con su procedencia.
type Fact struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

// Artifact es material técnico listo para el guion: código, benchmark, diagrama o referencia.
type Artifact struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Source  string `json:"source"`
}

// Validate comprueba las invariantes del dossier.
func (d Dossier) Validate() error {
	var problems []string
	if strings.TrimSpace(d.SchemaVersion) == "" {
		problems = append(problems, "schema_version es obligatorio")
	}
	if strings.TrimSpace(d.CaseID) == "" {
		problems = append(problems, "case_id es obligatorio")
	}
	if strings.TrimSpace(d.Title) == "" {
		problems = append(problems, "title es obligatorio")
	}
	if strings.TrimSpace(d.Summary) == "" {
		problems = append(problems, "summary es obligatorio")
	}
	if len(d.Facts) == 0 {
		problems = append(problems, "facts no puede estar vacío")
	}
	if len(d.Artifacts) == 0 {
		problems = append(problems, "artifacts no puede estar vacío")
	}
	for i, fact := range d.Facts {
		if strings.TrimSpace(fact.Label) == "" || strings.TrimSpace(fact.Value) == "" {
			problems = append(problems, fmt.Sprintf("facts[%d] requiere label y value", i))
		}
	}
	for i, artifact := range d.Artifacts {
		if strings.TrimSpace(artifact.ID) == "" || strings.TrimSpace(artifact.Kind) == "" ||
			strings.TrimSpace(artifact.Title) == "" || strings.TrimSpace(artifact.Content) == "" {
			problems = append(problems, fmt.Sprintf("artifacts[%d] requiere id, kind, title y content", i))
		}
	}
	if d.GeneratedAt.IsZero() {
		problems = append(problems, "generated_at es obligatorio")
	}
	if len(problems) > 0 {
		return fmt.Errorf("dossier_tecnico inválido: %s", strings.Join(problems, "; "))
	}
	return nil
}
