package atlas_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"elpanteon/engine/internal/atlas"
	"elpanteon/engine/internal/testutil"
)

func TestGitHubBuilderBuildsRealDossier(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/repos/acme/service":
			fmt.Fprint(writer, `{
				"full_name":"acme/service",
				"description":"Payment service",
				"language":"Go",
				"stargazers_count":42,
				"default_branch":"main",
				"html_url":"https://github.com/acme/service"
			}`)
		case "/repos/acme/service/git/trees/main":
			fmt.Fprint(writer, `{"tree":[
				{"path":"main.go","type":"blob","size":120},
				{"path":"internal/pool.go","type":"blob","size":240},
				{"path":"node_modules/x.js","type":"blob","size":100},
				{"path":"logo.png","type":"blob","size":100},
				{"path":"docs/guide.md","type":"blob","size":100}
			]}`)
		case "/repos/acme/service/issues":
			fmt.Fprint(writer, `[
				{"number":7,"title":"Pool exhaustion under load","state":"closed","body":"The pool ran out of connections.","html_url":"https://github.com/acme/service/issues/7"},
				{"number":8,"title":"PR title","state":"open","body":"should be ignored","pull_request":{}}
			]`)
		case "/raw/acme/service/main/README.md":
			writer.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(writer, "# Service\n\nPayment service with a connection pool.")
		case "/raw/acme/service/main/main.go":
			writer.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(writer, "package main\n\nfunc main() {}\n")
		case "/raw/acme/service/main/internal/pool.go":
			writer.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(writer, "package internal\n\nconst MaxConnections = 20\n")
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	builder := atlas.GitHubBuilder{
		Repo:    "acme/service",
		APIBase: server.URL,
		RawBase: server.URL + "/raw",
	}
	dossier, err := builder.Build(context.Background(), testutil.Case())
	if err != nil {
		t.Fatalf("Build(): %v", err)
	}
	if err := dossier.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}

	facts := map[string]string{}
	for _, fact := range dossier.Facts {
		facts[fact.Label] = fact.Value
	}
	if facts["Repositorio"] != "acme/service" {
		t.Fatalf("Repositorio = %q", facts["Repositorio"])
	}
	if facts["Lenguaje principal"] != "Go" {
		t.Fatalf("Lenguaje = %q", facts["Lenguaje principal"])
	}
	if facts["Síntoma"] == "" || facts["Solución"] == "" {
		t.Fatal("faltan facts del caso de ingeniería")
	}

	artifacts := map[string]string{}
	for _, artifact := range dossier.Artifacts {
		artifacts[artifact.ID] = artifact.Content
	}
	if !strings.Contains(artifacts["repo-code"], "MaxConnections") {
		t.Fatalf("repo-code = %q", artifacts["repo-code"])
	}
	if strings.Contains(artifacts["repo-code"], "node_modules") {
		t.Fatal("no debía incluir node_modules")
	}
	if !strings.Contains(artifacts["repo-issues"], "Pool exhaustion") {
		t.Fatalf("repo-issues = %q", artifacts["repo-issues"])
	}
	if strings.Contains(artifacts["repo-issues"], "PR title") {
		t.Fatal("no debía incluir pull requests")
	}
	if !strings.Contains(artifacts["repo-readme"], "Payment service") {
		t.Fatalf("repo-readme = %q", artifacts["repo-readme"])
	}
}

func TestInferRepo(t *testing.T) {
	cases := map[string]string{
		"https://github.com/acme/service/issues/7": "acme/service",
		"https://github.com/acme/service":          "acme/service",
		"https://example.com/acme/service":         "",
		"not a url":                                "",
	}
	for input, expected := range cases {
		if got := atlas.InferRepo(input); got != expected {
			t.Errorf("InferRepo(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestGitHubBuilderRequiresRepo(t *testing.T) {
	_, err := (atlas.GitHubBuilder{}).Build(context.Background(), testutil.Case())
	if err == nil {
		t.Fatal("Build() debía fallar sin repositorio")
	}
}

func TestGitHubIssueJSONDecodes(t *testing.T) {
	var issues []struct {
		Title       string    `json:"title"`
		PullRequest *struct{} `json:"pull_request"`
	}
	if err := json.Unmarshal([]byte(`[{"title":"x","pull_request":{}}]`), &issues); err != nil {
		t.Fatalf("Unmarshal(): %v", err)
	}
	if issues[0].PullRequest == nil {
		t.Fatal("pull_request no se decodificó")
	}
}
