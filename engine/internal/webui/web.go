package webui

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"time"
)

//go:embed static/* seed/*
var staticFiles embed.FS

func staticHandler() http.Handler {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}

func staticFile(name, contentType string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		body, err := staticFiles.ReadFile("static/" + name)
		if err != nil {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", contentType)
		writer.Header().Set("Cache-Control", "no-cache")
		if name == "sw.js" {
			writer.Header().Set("Service-Worker-Allowed", "/")
		}
		http.ServeContent(writer, request, name, time.Time{}, bytes.NewReader(body))
	}
}
