package galileo_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"elpanteon/engine/internal/galileo"
	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/galileo/extract"
	"elpanteon/engine/internal/galileo/source"
)

func fixture(t *testing.T) source.Document {
	t.Helper()
	doc, err := source.FromFile(filepath.Join("testdata", "postmortem.md"), source.Options{})
	if err != nil {
		t.Fatalf("FromFile(): %v", err)
	}
	return doc
}

func TestRunProducesValidArtifact(t *testing.T) {
	output := t.TempDir()
	service := galileo.New(extract.MarkedExtractor{}, output)
	service.Now = func() time.Time {
		return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	}

	result, err := service.Run(context.Background(), fixture(t))
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if result.Path == "" {
		t.Fatal("Path vacío")
	}
	if result.Case.HypeFilter.Verdict != contract.VerdictAccept {
		t.Fatalf("verdict = %s, flags = %+v", result.Case.HypeFilter.Verdict, result.Case.HypeFilter.Flags)
	}
	if err := result.Case.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}

	body, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("ReadFile(): %v", err)
	}
	var decoded contract.EngineeringCase
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("Unmarshal(): %v", err)
	}
	if decoded.ID != result.Case.ID {
		t.Fatalf("id = %q, want %q", decoded.ID, result.Case.ID)
	}
	if decoded.SchemaVersion != contract.SchemaVersion {
		t.Fatalf("schema_version = %q", decoded.SchemaVersion)
	}
}

func TestRunIsIdempotent(t *testing.T) {
	service := galileo.New(extract.MarkedExtractor{}, t.TempDir())
	service.Now = func() time.Time {
		return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	}

	first, err := service.Run(context.Background(), fixture(t))
	if err != nil {
		t.Fatalf("primer Run(): %v", err)
	}
	second, err := service.Run(context.Background(), fixture(t))
	if err != nil {
		t.Fatalf("segundo Run(): %v", err)
	}
	if first.Path != second.Path {
		t.Fatalf("paths distintos: %q != %q", first.Path, second.Path)
	}
	if first.CacheKey != second.CacheKey {
		t.Fatalf("cache keys distintas: %q != %q", first.CacheKey, second.CacheKey)
	}
}

func TestRunWrapsMissingSections(t *testing.T) {
	service := galileo.New(extract.MarkedExtractor{}, t.TempDir())
	doc := source.FromText("Texto sin la tríada.", source.Options{})

	_, err := service.Run(context.Background(), doc)
	if !errors.Is(err, extract.ErrMissingSections) {
		t.Fatalf("err = %v, want ErrMissingSections", err)
	}
}
