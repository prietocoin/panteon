package cervantes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"elpanteon/engine/internal/atlas"
	"elpanteon/engine/internal/cervantes"
	"elpanteon/engine/internal/testutil"
)

func TestLLMWriterProducesScenes(t *testing.T) {
	script := map[string]any{
		"scenes": []any{
			map[string]any{
				"title":     "El síntoma",
				"narration": strings.Repeat("La API devolvió errores y la latencia subió. ", 20),
				"beats": []any{
					map[string]any{"start": 0, "end": 0.4, "kind": "title", "title": "El síntoma"},
					map[string]any{"start": 0.4, "end": 1, "kind": "bullets", "title": "Impacto", "bullets": []string{"503", "p99 4200 ms"}},
				},
			},
			map[string]any{
				"title":     "La causa",
				"narration": strings.Repeat("El pool se agotó porque una migración no liberó conexiones. ", 20),
				"beats":     []any{map[string]any{"start": 0, "end": 1, "kind": "title", "title": "La causa"}},
			},
		},
	}
	content, _ := json.Marshal(script)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{"content": string(content)},
			}},
		})
	}))
	defer server.Close()

	dossier, err := (atlas.DeterministicBuilder{}).Build(context.Background(), testutil.Case())
	if err != nil {
		t.Fatalf("Build(): %v", err)
	}
	writer := &cervantes.LLMWriter{
		BaseURL: server.URL + "/v1",
		Model:   "test-model",
		APIKey:  "test-key",
	}
	cut, err := cervantes.Write(dossier, cervantes.Options{
		SceneMode:     true,
		TargetMinutes: 12,
		LLM:           writer,
		Context:       context.Background(),
	})
	if err != nil {
		t.Fatalf("Write(): %v", err)
	}
	if len(cut.Scenes) != 2 {
		t.Fatalf("scenes = %d, want 2", len(cut.Scenes))
	}
	if len(cut.Scenes[0].Clips[0].Visual.Beats) != 2 {
		t.Fatalf("beats = %d", len(cut.Scenes[0].Clips[0].Visual.Beats))
	}
	if cut.Scenes[0].Clips[0].DurationSec <= 0 {
		t.Fatal("duration inválida")
	}
}

func TestLLMFailureFallsBackToDeterministic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	dossier, err := (atlas.DeterministicBuilder{}).Build(context.Background(), testutil.Case())
	if err != nil {
		t.Fatalf("Build(): %v", err)
	}
	cut, err := cervantes.Write(dossier, cervantes.Options{
		SceneMode: true,
		LLM: &cervantes.LLMWriter{
			BaseURL: server.URL + "/v1",
			Model:   "test",
			APIKey:  "test",
		},
		Context: context.Background(),
	})
	if err != nil {
		t.Fatalf("Write(): %v", err)
	}
	if len(cut.Scenes) != 6 {
		t.Fatalf("fallback scenes = %d, want 6", len(cut.Scenes))
	}
}
