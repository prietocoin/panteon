package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"elpanteon/engine/internal/galileo/contract"
)

func TestPipelineExamplesMatchContracts(t *testing.T) {
	base := filepath.Join("..", "..", "..", "..", "shared", "contracts")

	var engineeringCase contract.EngineeringCase
	readJSON(t, filepath.Join(base, "engineering_case.example.json"), &engineeringCase)
	if err := engineeringCase.Validate(); err != nil {
		t.Fatalf("engineering_case: %v", err)
	}

	var dossier contract.Dossier
	readJSON(t, filepath.Join(base, "dossier_tecnico.example.json"), &dossier)
	if err := dossier.Validate(); err != nil {
		t.Fatalf("dossier: %v", err)
	}

	var cut contract.DirectorCut
	readJSON(t, filepath.Join(base, "director_cut.example.json"), &cut)
	if err := cut.Validate(); err != nil {
		t.Fatalf("director_cut: %v", err)
	}

	var manifest contract.RunManifest
	readJSON(t, filepath.Join(base, "run_manifest.example.json"), &manifest)
	if err := manifest.Validate(); err != nil {
		t.Fatalf("run_manifest: %v", err)
	}
}

func readJSON(t *testing.T, path string, target any) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("Unmarshal(%s): %v", path, err)
	}
}
