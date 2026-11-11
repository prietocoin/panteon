package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/galileo/store"
)

func TestFileNameSanitizesID(t *testing.T) {
	got := store.FileName("sha256:abc/../def")
	want := "sha256-abc-..-def.json"
	if got != want {
		t.Fatalf("FileName() = %q, want %q", got, want)
	}
}

func TestCacheKeyIsStable(t *testing.T) {
	first := store.CacheKey("hash", "model", "v1")
	second := store.CacheKey("hash", "model", "v1")
	if first != second {
		t.Fatalf("CacheKey() inestable: %q != %q", first, second)
	}
	if first == store.CacheKey("hash", "model", "v2") {
		t.Fatal("CacheKey() no cambia con la versión de prompt")
	}
}

func TestSaveWritesArtifact(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cases")
	engineeringCase := contract.EngineeringCase{
		SchemaVersion: contract.SchemaVersion,
		ID:            "sha256:test",
		Title:         "Caso de prueba",
		GeneratedAt:   time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC),
	}

	path, err := (store.Store{Dir: dir}).Save(engineeringCase)
	if err != nil {
		t.Fatalf("Save(): %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("directorio = %q, want %q", filepath.Dir(path), dir)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(): %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("artefacto vacío")
	}
}
