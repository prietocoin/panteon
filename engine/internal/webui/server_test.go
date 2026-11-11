package webui_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"elpanteon/engine/internal/webui"
)

const sampleSource = `# Incident: cache stampede

## Symptom
La API devolvió errores 500 durante 8 minutos y la latencia p99 llegó a 3000 ms.

## Root cause
Una cache key expiró para todos los usuarios y generó 5000 consultas por segundo.

## Solution
Se añadió jitter al TTL y single-flight por clave.
- Aplicar jitter de 0 a 30 segundos.
- Serializar la regeneración por clave.

## Tradeoffs
- Algunas claves viven hasta 30 segundos más.

## Verification
La latencia p99 volvió a 120 ms durante 24 horas.
`

func TestAPIEndToEnd(t *testing.T) {
	server, err := webui.New(t.TempDir())
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	created := postJSON(t, httpServer.URL+"/api/cases", map[string]any{
		"text":  sampleSource,
		"title": "Incident: cache stampede",
		"type":  "postmortem",
	})
	publicID, _ := created["id"].(string)
	if publicID == "" {
		t.Fatalf("id vacío: %#v", created)
	}

	detail := getJSON(t, httpServer.URL+"/api/cases/"+publicID)
	if detail["case"] == nil {
		t.Fatalf("caso no devuelto: %#v", detail)
	}

	postJSON(t, httpServer.URL+"/api/cases/"+publicID+"/review", map[string]any{
		"decision": "approved",
		"note":     "test",
	})

	job := postJSON(t, httpServer.URL+"/api/cases/"+publicID+"/render", map[string]any{
		"clip_seconds": 0.25,
		"clip_copies":  1,
		"fps":          4,
		"width":        160,
		"height":       90,
		"renderer":     "raster",
		"encoder":      "gif",
		"tts":          "tone",
		"atlas":        "deterministic",
	})
	jobID, _ := job["id"].(string)
	if jobID == "" {
		t.Fatalf("job sin id: %#v", job)
	}

	deadline := time.Now().Add(20 * time.Second)
	var final map[string]any
	for time.Now().Before(deadline) {
		final = getJSON(t, httpServer.URL+"/api/jobs/"+jobID)
		status, _ := final["status"].(string)
		if status == "done" || status == "failed" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if final["status"] != "done" {
		t.Fatalf("job no terminó: %#v", final)
	}

	outputs, _ := final["outputs"].([]any)
	if len(outputs) == 0 {
		t.Fatal("job sin outputs")
	}
	video := ""
	for _, item := range outputs {
		output, _ := item.(map[string]any)
		if output["kind"] == "video" {
			video, _ = output["path"].(string)
		}
	}
	if video == "" {
		t.Fatalf("sin vídeo: %#v", outputs)
	}

	response, err := http.Get(httpServer.URL + "/api/file?path=" + strings.ReplaceAll(video, "/", "%2F"))
	if err != nil {
		t.Fatalf("GET file: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("file status = %d", response.StatusCode)
	}

	traversal, err := http.Get(httpServer.URL + "/api/file?path=..%2F..%2Fsecret")
	if err != nil {
		t.Fatalf("GET traversal: %v", err)
	}
	defer traversal.Body.Close()
	if traversal.StatusCode != http.StatusBadRequest {
		t.Fatalf("traversal status = %d", traversal.StatusCode)
	}
}

func getJSON(t *testing.T, url string) map[string]any {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer response.Body.Close()
	return decodeResponse(t, response)
}

func postJSON(t *testing.T, url string, payload any) map[string]any {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	response, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer response.Body.Close()
	return decodeResponse(t, response)
}

func decodeResponse(t *testing.T, response *http.Response) map[string]any {
	t.Helper()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		t.Fatalf("status %d", response.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return payload
}
