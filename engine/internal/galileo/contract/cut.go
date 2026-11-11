package contract

import (
	"fmt"
	"strings"
	"time"
)

// CutSchemaVersion identifica la versión de director_cut.json.
const CutSchemaVersion = "1.0"

// DirectorCut es la salida de Cervantes y la entrada de Da Vinci y Pixar.
type DirectorCut struct {
	SchemaVersion string    `json:"schema_version"`
	CaseID        string    `json:"case_id"`
	Title         string    `json:"title"`
	Scenes        []Scene   `json:"scenes"`
	GeneratedAt   time.Time `json:"generated_at"`
}

// Scene agrupa clips con continuidad atmosférica.
type Scene struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Clips []Clip `json:"clips"`
}

// Clip es la unidad atómica de renderizado.
type Clip struct {
	ID          string   `json:"id"`
	Narration   string   `json:"narration"`
	DurationSec float64  `json:"duration_sec"`
	Visual      Visual   `json:"visual"`
	SFX         []string `json:"sfx,omitempty"`
}

// Visual describe la intención gráfica que Da Vinci debe materializar.
type Visual struct {
	Kind         string   `json:"kind"`
	Title        string   `json:"title"`
	Subtitle     string   `json:"subtitle,omitempty"`
	Bullets      []string `json:"bullets,omitempty"`
	Cards        []Card   `json:"cards,omitempty"`
	Code         string   `json:"code,omitempty"`
	CodeLanguage string   `json:"code_language,omitempty"`
	Beats        []Beat   `json:"beats,omitempty"`
}

// Beat es un tramo visual dentro de una escena de duración variable.
// Start y End son fracciones de 0 a 1 de la escena.
type Beat struct {
	Start        float64  `json:"start"`
	End          float64  `json:"end"`
	Kind         string   `json:"kind"`
	Title        string   `json:"title"`
	Subtitle     string   `json:"subtitle,omitempty"`
	Bullets      []string `json:"bullets,omitempty"`
	Cards        []Card   `json:"cards,omitempty"`
	Code         string   `json:"code,omitempty"`
	CodeLanguage string   `json:"code_language,omitempty"`
}

// Card es una primitiva atómica de comparación o estado.
type Card struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Tone  string `json:"tone"` // danger, success, neutral, warning
}

// Validate comprueba las invariantes del guion.
func (c DirectorCut) Validate() error {
	var problems []string
	if strings.TrimSpace(c.SchemaVersion) == "" {
		problems = append(problems, "schema_version es obligatorio")
	}
	if strings.TrimSpace(c.CaseID) == "" {
		problems = append(problems, "case_id es obligatorio")
	}
	if strings.TrimSpace(c.Title) == "" {
		problems = append(problems, "title es obligatorio")
	}
	if len(c.Scenes) == 0 {
		problems = append(problems, "scenes no puede estar vacío")
	}
	for i, scene := range c.Scenes {
		if strings.TrimSpace(scene.ID) == "" || strings.TrimSpace(scene.Title) == "" {
			problems = append(problems, fmt.Sprintf("scenes[%d] requiere id y title", i))
		}
		if len(scene.Clips) == 0 {
			problems = append(problems, fmt.Sprintf("scenes[%d] no tiene clips", i))
		}
		for j, clip := range scene.Clips {
			if strings.TrimSpace(clip.ID) == "" {
				problems = append(problems, fmt.Sprintf("scenes[%d].clips[%d].id es obligatorio", i, j))
			}
			if strings.TrimSpace(clip.Narration) == "" {
				problems = append(problems, fmt.Sprintf("scenes[%d].clips[%d].narration es obligatorio", i, j))
			}
			if clip.DurationSec <= 0 {
				problems = append(problems, fmt.Sprintf("scenes[%d].clips[%d].duration_sec debe ser > 0", i, j))
			}
			if strings.TrimSpace(clip.Visual.Kind) == "" || strings.TrimSpace(clip.Visual.Title) == "" {
				if len(clip.Visual.Beats) == 0 {
					problems = append(problems, fmt.Sprintf("scenes[%d].clips[%d].visual requiere kind y title", i, j))
				}
			}
			for k, beat := range clip.Visual.Beats {
				if beat.Start < 0 || beat.End > 1 || beat.Start >= beat.End {
					problems = append(problems, fmt.Sprintf("scenes[%d].clips[%d].visual.beats[%d] tiene tiempos inválidos", i, j, k))
				}
				if strings.TrimSpace(beat.Kind) == "" || strings.TrimSpace(beat.Title) == "" {
					problems = append(problems, fmt.Sprintf("scenes[%d].clips[%d].visual.beats[%d] requiere kind y title", i, j, k))
				}
			}
		}
	}
	if c.GeneratedAt.IsZero() {
		problems = append(problems, "generated_at es obligatorio")
	}
	if len(problems) > 0 {
		return fmt.Errorf("director_cut inválido: %s", strings.Join(problems, "; "))
	}
	return nil
}
