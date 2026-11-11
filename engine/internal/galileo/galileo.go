package galileo

import (
	"context"
	"fmt"
	"time"

	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/galileo/extract"
	"elpanteon/engine/internal/galileo/filter"
	"elpanteon/engine/internal/galileo/source"
	"elpanteon/engine/internal/galileo/store"
)

// Service orquesta extracción, filtro Anti-Hype y persistencia.
type Service struct {
	Extractor     extract.Extractor
	Store         store.Store
	Now           func() time.Time
	Model         string
	PromptVersion string
}

// Result es el artefacto producido por una ejecución de Galileo.
type Result struct {
	Case     contract.EngineeringCase
	Path     string
	CacheKey string
}

// New construye un servicio con el extractor indicado y el directorio de salida.
func New(extractor extract.Extractor, outputDir string) *Service {
	return &Service{
		Extractor:     extractor,
		Store:         store.Store{Dir: outputDir},
		Now:           time.Now,
		Model:         "marked-v1",
		PromptVersion: "v1",
	}
}

// Run ejecuta el pipeline de Galileo sobre un documento normalizado.
func (s *Service) Run(ctx context.Context, doc source.Document) (Result, error) {
	if s.Extractor == nil {
		return Result{}, fmt.Errorf("galileo: extractor no configurado")
	}

	now := time.Now
	if s.Now != nil {
		now = s.Now
	}

	draft, err := s.Extractor.Extract(ctx, doc)
	if err != nil {
		return Result{}, fmt.Errorf("galileo: extracción: %w", err)
	}

	hype := filter.Evaluate(draft)
	engineeringCase := contract.EngineeringCase{
		SchemaVersion: contract.SchemaVersion,
		ID:            doc.ContentHash,
		Title:         firstNonEmpty(draft.Title, doc.Title, "Caso de ingeniería sin título"),
		Source: contract.Source{
			URL:         doc.URL,
			Type:        doc.Type,
			Title:       doc.Title,
			RetrievedAt: doc.RetrievedAt,
			ContentHash: doc.ContentHash,
		},
		Symptom:     draft.Symptom,
		Cause:       draft.Cause,
		Solution:    draft.Solution,
		HypeFilter:  hype,
		Entities:    draft.Entities,
		GeneratedAt: now().UTC(),
	}

	engineeringCase.Symptom.Evidence = nonNilEvidence(engineeringCase.Symptom.Evidence)
	engineeringCase.Cause.Evidence = nonNilEvidence(engineeringCase.Cause.Evidence)
	engineeringCase.Solution.Evidence = nonNilEvidence(engineeringCase.Solution.Evidence)
	if engineeringCase.Solution.Actions == nil {
		engineeringCase.Solution.Actions = []contract.Action{}
	}
	if engineeringCase.Solution.Tradeoffs == nil {
		engineeringCase.Solution.Tradeoffs = []string{}
	}
	if engineeringCase.Source.Type == "" {
		engineeringCase.Source.Type = "other"
	}
	if engineeringCase.Source.RetrievedAt.IsZero() {
		engineeringCase.Source.RetrievedAt = engineeringCase.GeneratedAt
	}
	if engineeringCase.ID == "" {
		engineeringCase.ID = contract.ContentHash(doc.Text)
	}
	if engineeringCase.Source.ContentHash == "" {
		engineeringCase.Source.ContentHash = engineeringCase.ID
	}

	if err := engineeringCase.Validate(); err != nil {
		return Result{}, err
	}

	result := Result{
		Case:     engineeringCase,
		CacheKey: store.CacheKey(doc.ContentHash, s.Model, s.PromptVersion),
	}
	if s.Store.Dir != "" {
		path, err := s.Store.Save(engineeringCase)
		if err != nil {
			return Result{}, fmt.Errorf("galileo: guardando caso: %w", err)
		}
		result.Path = path
	}
	return result, nil
}

func nonNilEvidence(items []contract.Evidence) []contract.Evidence {
	if items == nil {
		return []contract.Evidence{}
	}
	return items
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
