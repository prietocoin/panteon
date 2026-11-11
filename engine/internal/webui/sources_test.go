package webui_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"elpanteon/engine/internal/webui"
)

func TestSourcesAPI(t *testing.T) {
	server, err := webui.New(t.TempDir())
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	sourceItem := postJSON(t, httpServer.URL+"/api/sources", map[string]any{
		"type":             "rss",
		"name":             "Blog de ingeniería",
		"url":              "https://example.com/feed.xml",
		"interval_minutes": 30,
		"auto_create":      true,
		"enabled":          true,
	})
	id, _ := sourceItem["id"].(string)
	if id == "" {
		t.Fatalf("fuente sin id: %#v", sourceItem)
	}

	list := getJSON(t, httpServer.URL+"/api/sources")
	sources, _ := list["sources"].([]any)
	if len(sources) != 1 {
		t.Fatalf("sources = %d", len(sources))
	}

	request, err := http.NewRequest(http.MethodDelete, httpServer.URL+"/api/sources/"+id, nil)
	if err != nil {
		t.Fatalf("NewRequest(): %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d", response.StatusCode)
	}

	list = getJSON(t, httpServer.URL+"/api/sources")
	sources, _ = list["sources"].([]any)
	if len(sources) != 0 {
		t.Fatalf("sources = %d después de borrar", len(sources))
	}
}
