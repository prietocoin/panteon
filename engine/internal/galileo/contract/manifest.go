package contract

import (
	"strings"
	"time"
)

// ManifestSchemaVersion identifica la versión de run_manifest.json.
const ManifestSchemaVersion = "1.0"

// RunManifest resume una ejecución completa del pipeline.
type RunManifest struct {
	SchemaVersion string         `json:"schema_version"`
	CaseID        string         `json:"case_id"`
	Title         string         `json:"title"`
	StartedAt     time.Time      `json:"started_at"`
	FinishedAt    time.Time      `json:"finished_at"`
	FPS           int            `json:"fps"`
	Width         int            `json:"width"`
	Height        int            `json:"height"`
	FrameCount    int            `json:"frame_count"`
	Rigor         RigorReport    `json:"rigor"`
	Clips         []ClipManifest `json:"clips"`
	Outputs       []Output       `json:"outputs"`
}

// ClipManifest describe el resultado de un clip atómico.
type ClipManifest struct {
	ID          string  `json:"id"`
	Narration   string  `json:"narration"`
	DurationSec float64 `json:"duration_sec"`
	Frames      int     `json:"frames"`
	HTML        string  `json:"html,omitempty"`
	Audio       string  `json:"audio,omitempty"`
	CacheKey    string  `json:"cache_key"`
	Reused      bool    `json:"reused"`
}

// Output es un artefacto final de la ejecución.
type Output struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

// RigorReport es el veredicto de Sócrates.
type RigorReport struct {
	Passed   bool           `json:"passed"`
	Score    int            `json:"score"`
	Findings []RigorFinding `json:"findings"`
}

// RigorFinding es una observación de Sócrates.
type RigorFinding struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// Validate comprueba las invariantes mínimas del manifiesto.
func (m RunManifest) Validate() error {
	if strings.TrimSpace(m.SchemaVersion) == "" ||
		strings.TrimSpace(m.CaseID) == "" ||
		strings.TrimSpace(m.Title) == "" ||
		m.FPS <= 0 || m.Width <= 0 || m.Height <= 0 ||
		m.StartedAt.IsZero() || m.FinishedAt.IsZero() ||
		len(m.Clips) == 0 || len(m.Outputs) == 0 {
		return errManifest
	}
	return nil
}

var errManifest = &manifestError{}

type manifestError struct{}

func (*manifestError) Error() string { return "run_manifest inválido" }
