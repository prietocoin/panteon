package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"elpanteon/engine/internal/galileo/contract"
)

func TestExampleArtifactMatchesContract(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "shared", "contracts", "engineering_case.example.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}

	var engineeringCase contract.EngineeringCase
	if err := json.Unmarshal(body, &engineeringCase); err != nil {
		t.Fatalf("Unmarshal(): %v", err)
	}
	if err := engineeringCase.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	if engineeringCase.SchemaVersion != contract.SchemaVersion {
		t.Fatalf("schema_version = %q", engineeringCase.SchemaVersion)
	}
}
