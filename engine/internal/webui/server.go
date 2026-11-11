package webui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"elpanteon/engine/internal/atlas"
	"elpanteon/engine/internal/cervantes"
	"elpanteon/engine/internal/encoder"
	"elpanteon/engine/internal/galileo"
	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/galileo/extract"
	"elpanteon/engine/internal/galileo/source"
	"elpanteon/engine/internal/galileo/store"
	"elpanteon/engine/internal/ingest"
	"elpanteon/engine/internal/keepawake"
	"elpanteon/engine/internal/pavarotti"
	"elpanteon/engine/internal/pipeline"
	"elpanteon/engine/internal/pixar"
)

// Server expone la API HTTP y sirve la UI embebida.
type Server struct {
	workspace  string
	casesDir   string
	reviewsDir string
	runsDir    string
	jobs       *JobManager
	ingest     *ingest.Service
}

// New prepara el workspace y siembra el caso demo si está vacío.
func New(workspace string) (*Server, error) {
	absolute, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	server := &Server{
		workspace:  absolute,
		casesDir:   filepath.Join(absolute, "cases"),
		reviewsDir: filepath.Join(absolute, "reviews"),
		runsDir:    filepath.Join(absolute, "runs"),
		jobs:       NewJobManager(),
	}
	for _, dir := range []string{server.workspace, server.casesDir, server.reviewsDir, server.runsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	service, err := ingest.New(
		filepath.Join(absolute, "ingest"),
		extract.MarkedExtractor{},
		server.saveCase,
		ingest.NotifyFromEnv(),
	)
	if err != nil {
		return nil, err
	}
	server.ingest = service
	if strings.ToLower(strings.TrimSpace(os.Getenv("PANTEON_SEED"))) != "false" {
		server.seed()
	}
	return server, nil
}

// Start arranca el watcher de fuentes.
func (s *Server) Start(ctx context.Context) {
	if s.ingest != nil {
		s.ingest.Start(ctx)
	}
}

// Handler devuelve el router HTTP completo.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/summary", s.handleSummary)
	mux.HandleFunc("GET /api/cases", s.handleListCases)
	mux.HandleFunc("POST /api/cases", s.handleCreateCase)
	mux.HandleFunc("GET /api/cases/{id}", s.handleGetCase)
	mux.HandleFunc("POST /api/cases/{id}/review", s.handleReview)
	mux.HandleFunc("POST /api/cases/{id}/render", s.handleRender)
	mux.HandleFunc("GET /api/jobs", s.handleListJobs)
	mux.HandleFunc("GET /api/jobs/{id}", s.handleGetJob)
	mux.HandleFunc("GET /api/jobs/{id}/events", s.handleJobEvents)
	mux.HandleFunc("GET /api/sources", s.handleSources)
	mux.HandleFunc("POST /api/sources", s.handleAddSource)
	mux.HandleFunc("DELETE /api/sources/{id}", s.handleDeleteSource)
	mux.HandleFunc("POST /api/sources/check", s.handleCheckSources)
	mux.HandleFunc("GET /api/ingest/events", s.handleIngestEvents)
	mux.HandleFunc("GET /api/file", s.handleFile)
	mux.Handle("GET /manifest.webmanifest", staticFile("manifest.webmanifest", "application/manifest+json"))
	mux.Handle("GET /sw.js", staticFile("sw.js", "application/javascript"))
	mux.HandleFunc("/api/", func(writer http.ResponseWriter, request *http.Request) {
		writeError(writer, http.StatusNotFound, "endpoint no encontrado")
	})
	mux.Handle("/", staticHandler())
	return mux
}

type summaryResponse struct {
	Cases         int    `json:"cases"`
	Jobs          int    `json:"jobs"`
	ActiveJobs    int    `json:"active_jobs"`
	FFmpeg        bool   `json:"ffmpeg"`
	Chrome        bool   `json:"chrome"`
	Workspace     string `json:"workspace"`
	TTSURL        string `json:"tts_url"`
	TTSConfigured bool   `json:"tts_configured"`
	LLM           bool   `json:"llm"`
	LLMModel      string `json:"llm_model"`
}

func (s *Server) handleHealth(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{"ok": true, "version": "0.2.0"})
}

func (s *Server) handleSummary(writer http.ResponseWriter, _ *http.Request) {
	cases, _ := s.listCases()
	ttsURL := strings.TrimSpace(os.Getenv("PANTEON_TTS_URL"))
	ttsConfigured := ttsURL != ""
	if ttsURL == "" {
		ttsURL = "http://localhost:8000"
	}
	llm := cervantes.NewLLMWriterFromEnv()
	writeJSON(writer, http.StatusOK, summaryResponse{
		Cases:         len(cases),
		Jobs:          len(s.jobs.list()),
		ActiveJobs:    s.jobs.activeCount(),
		FFmpeg:        encoder.Select("auto").Name() == "ffmpeg",
		Chrome:        pixar.FindChrome() != "",
		Workspace:     s.workspace,
		TTSURL:        ttsURL,
		TTSConfigured: ttsConfigured,
		LLM:           llm != nil,
		LLMModel:      os.Getenv("PANTEON_LLM_MODEL"),
	})
}

type caseSummary struct {
	ID          string    `json:"id"`
	CaseID      string    `json:"case_id"`
	Title       string    `json:"title"`
	Verdict     string    `json:"verdict"`
	Score       int       `json:"score"`
	Review      string    `json:"review"`
	GeneratedAt time.Time `json:"generated_at"`
	Runs        int       `json:"runs"`
}

type reviewRecord struct {
	Decision  string    `json:"decision"`
	Note      string    `json:"note"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type runClip struct {
	ID     string `json:"id"`
	HTML   string `json:"html,omitempty"`
	Audio  string `json:"audio,omitempty"`
	Frames int    `json:"frames"`
}

type runSummary struct {
	ID         string      `json:"id"`
	FinishedAt time.Time   `json:"finished_at"`
	Frames     int         `json:"frames"`
	Outputs    []JobOutput `json:"outputs"`
	Clips      []runClip   `json:"clips,omitempty"`
}

type caseDetail struct {
	ID     string                   `json:"id"`
	Case   contract.EngineeringCase `json:"case"`
	Review reviewRecord             `json:"review"`
	Runs   []runSummary             `json:"runs"`
}

func (s *Server) handleListCases(writer http.ResponseWriter, _ *http.Request) {
	cases, err := s.listCases()
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, cases)
}

type createCaseRequest struct {
	URL   string `json:"url"`
	Text  string `json:"text"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

func (s *Server) handleCreateCase(writer http.ResponseWriter, request *http.Request) {
	var payload createCaseRequest
	if err := decodeJSON(request, &payload); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 60*time.Second)
	defer cancel()

	var (
		document source.Document
		err      error
	)
	switch {
	case strings.TrimSpace(payload.URL) != "":
		document, err = source.FromURL(ctx, strings.TrimSpace(payload.URL), nil)
	case strings.TrimSpace(payload.Text) != "":
		document = source.FromText(payload.Text, source.Options{
			Title: payload.Title,
			Type:  payload.Type,
		})
	default:
		writeError(writer, http.StatusBadRequest, "indica url o text")
		return
	}
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}

	service := galileo.New(extract.MarkedExtractor{}, filepath.Join(s.workspace, "seeds"))
	result, err := service.Run(ctx, document)
	if err != nil {
		writeError(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	publicID, err := s.saveCase(result.Case)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, map[string]any{
		"id":     publicID,
		"case":   result.Case,
		"review": reviewRecord{Decision: "pending"},
	})
}

func (s *Server) handleGetCase(writer http.ResponseWriter, request *http.Request) {
	publicID := request.PathValue("id")
	engineeringCase, err := s.loadCase(publicID)
	if err != nil {
		writeError(writer, http.StatusNotFound, "caso no encontrado")
		return
	}
	writeJSON(writer, http.StatusOK, caseDetail{
		ID:     publicID,
		Case:   engineeringCase,
		Review: s.readReview(publicID),
		Runs:   s.scanRuns(engineeringCase.ID),
	})
}

type reviewRequest struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

func (s *Server) handleReview(writer http.ResponseWriter, request *http.Request) {
	publicID := request.PathValue("id")
	if _, err := s.loadCase(publicID); err != nil {
		writeError(writer, http.StatusNotFound, "caso no encontrado")
		return
	}
	var payload reviewRequest
	if err := decodeJSON(request, &payload); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	switch payload.Decision {
	case "pending", "approved", "rejected":
	default:
		writeError(writer, http.StatusBadRequest, "decisión inválida")
		return
	}
	if len(payload.Note) > 2000 {
		writeError(writer, http.StatusBadRequest, "nota demasiado larga")
		return
	}
	record := reviewRecord{Decision: payload.Decision, Note: payload.Note, UpdatedAt: time.Now().UTC()}
	if err := s.writeReview(publicID, record); err != nil {
		writeError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, record)
}

type renderRequest struct {
	ClipSeconds   float64 `json:"clip_seconds"`
	ClipCopies    int     `json:"clip_copies"`
	FPS           int     `json:"fps"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	Renderer      string  `json:"renderer"`
	Encoder       string  `json:"encoder"`
	TTS           string  `json:"tts"`
	TTSURL        string  `json:"tts_url"`
	Voice         string  `json:"voice"`
	Atlas         string  `json:"atlas"`
	Repo          string  `json:"repo"`
	Ref           string  `json:"ref"`
	Chrome        string  `json:"chrome"`
	SceneMode     bool    `json:"scene_mode"`
	TargetMinutes int     `json:"target_minutes"`
}

func (s *Server) handleRender(writer http.ResponseWriter, request *http.Request) {
	publicID := request.PathValue("id")
	engineeringCase, err := s.loadCase(publicID)
	if err != nil {
		writeError(writer, http.StatusNotFound, "caso no encontrado")
		return
	}
	review := s.readReview(publicID)
	if review.Decision == "rejected" {
		writeError(writer, http.StatusConflict, "el caso está rechazado; cámbialo a aprobado o pendiente")
		return
	}
	var payload renderRequest
	if err := decodeJSON(request, &payload); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	job, err := s.startRender(engineeringCase, publicID, payload)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusAccepted, job)
}

func (s *Server) handleListJobs(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, s.jobs.list())
}

func (s *Server) handleGetJob(writer http.ResponseWriter, request *http.Request) {
	job, ok := s.jobs.get(request.PathValue("id"))
	if !ok {
		writeError(writer, http.StatusNotFound, "job no encontrado")
		return
	}
	writeJSON(writer, http.StatusOK, job)
}

func (s *Server) handleJobEvents(writer http.ResponseWriter, request *http.Request) {
	jobID := request.PathValue("id")
	if _, ok := s.jobs.get(jobID); !ok {
		writeError(writer, http.StatusNotFound, "job no encontrado")
		return
	}
	flusher, ok := writer.(http.Flusher)
	if !ok {
		writeError(writer, http.StatusInternalServerError, "streaming no soportado")
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")

	channel, cancel := s.jobs.subscribe(jobID)
	defer cancel()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case job := <-channel:
			body, _ := json.Marshal(job)
			fmt.Fprintf(writer, "data: %s\n\n", body)
			flusher.Flush()
			if job.Status == "done" || job.Status == "failed" {
				return
			}
		case <-ticker.C:
			fmt.Fprint(writer, ": ping\n\n")
			flusher.Flush()
		case <-request.Context().Done():
			return
		}
	}
}

type sourcePayload struct {
	Type            string   `json:"type"`
	Name            string   `json:"name"`
	URL             string   `json:"url"`
	Repo            string   `json:"repo"`
	IntervalMinutes int      `json:"interval_minutes"`
	AutoCreate      *bool    `json:"auto_create"`
	Enabled         *bool    `json:"enabled"`
	Keywords        []string `json:"keywords"`
}

func (s *Server) handleSources(writer http.ResponseWriter, _ *http.Request) {
	sources := s.ingest.ListSources()
	ingest.SortSources(sources)
	writeJSON(writer, http.StatusOK, map[string]any{
		"sources": sources,
		"events":  s.ingest.Events(),
	})
}

func (s *Server) handleAddSource(writer http.ResponseWriter, request *http.Request) {
	var payload sourcePayload
	if err := decodeJSON(request, &payload); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	autoCreate := true
	if payload.AutoCreate != nil {
		autoCreate = *payload.AutoCreate
	}
	sourceItem, err := s.ingest.AddSource(ingest.Source{
		Type:            payload.Type,
		Name:            payload.Name,
		URL:             payload.URL,
		Repo:            payload.Repo,
		IntervalMinutes: payload.IntervalMinutes,
		Enabled:         enabled,
		AutoCreate:      autoCreate,
		Keywords:        payload.Keywords,
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		_, _ = s.ingest.RunOnce(ctx, true)
	}()
	writeJSON(writer, http.StatusCreated, sourceItem)
}

func (s *Server) handleDeleteSource(writer http.ResponseWriter, request *http.Request) {
	if !s.ingest.DeleteSource(request.PathValue("id")) {
		writeError(writer, http.StatusNotFound, "fuente no encontrada")
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCheckSources(writer http.ResponseWriter, _ *http.Request) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		_, _ = s.ingest.RunOnce(ctx, true)
	}()
	writeJSON(writer, http.StatusAccepted, map[string]string{"status": "started"})
}

func (s *Server) handleIngestEvents(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, s.ingest.Events())
}

func (s *Server) handleFile(writer http.ResponseWriter, request *http.Request) {
	relative := strings.TrimSpace(request.URL.Query().Get("path"))
	if relative == "" {
		writeError(writer, http.StatusBadRequest, "falta path")
		return
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		writeError(writer, http.StatusBadRequest, "path inválido")
		return
	}
	full := filepath.Join(s.workspace, clean)
	rel, err := filepath.Rel(s.workspace, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		writeError(writer, http.StatusBadRequest, "path fuera del workspace")
		return
	}
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		writeError(writer, http.StatusNotFound, "archivo no encontrado")
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	http.ServeFile(writer, request, full)
}

func (s *Server) startRender(engineeringCase contract.EngineeringCase, publicID string, payload renderRequest) (*Job, error) {
	config, err := s.buildConfig(payload)
	if err != nil {
		return nil, err
	}
	jobID := fmt.Sprintf("job-%s-%s", time.Now().Format("20060102-150405"), randomHex(3))
	outDir := filepath.Join(s.runsDir, jobID)
	job := s.jobs.create(Job{
		ID:           jobID,
		Type:         "render",
		CaseID:       engineeringCase.ID,
		PublicCaseID: publicID,
		Title:        engineeringCase.Title,
		Status:       "queued",
		Phase:        "queued",
		Message:      "En cola",
		CreatedAt:    time.Now().UTC(),
	})
	go s.runRender(jobID, engineeringCase, config, outDir)
	return &job, nil
}

func (s *Server) runRender(jobID string, engineeringCase contract.EngineeringCase, config pipeline.Config, outDir string) {
	keepawake.Enable()
	defer keepawake.Disable()

	s.jobs.update(jobID, func(job *Job) {
		job.Status = "running"
		job.StartedAt = time.Now().UTC()
		job.Phase = "starting"
		job.Message = "Iniciando pipeline"
		job.Percent = 1
	})

	fail := func(err error) {
		s.jobs.update(jobID, func(job *Job) {
			job.Status = "failed"
			job.Error = err.Error()
			job.Message = "Render falló"
			job.FinishedAt = time.Now().UTC()
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()

	config.OutDir = outDir
	config.Progress = func(progress pipeline.Progress) {
		s.jobs.update(jobID, func(job *Job) {
			job.Phase = progress.Phase
			job.Message = progress.Message
			if progress.TotalClips > 0 {
				job.Clip = progress.Clip
				job.TotalClips = progress.TotalClips
			}
			job.Percent = progress.Percent
		})
	}

	service, err := pipeline.New(config)
	if err != nil {
		fail(err)
		return
	}
	manifest, err := service.Run(ctx, engineeringCase)
	if err != nil {
		fail(err)
		return
	}

	outputs := make([]JobOutput, 0, len(manifest.Outputs))
	for _, output := range manifest.Outputs {
		if relative := s.relPath(output.Path); relative != "" {
			outputs = append(outputs, JobOutput{Kind: output.Kind, Path: relative})
		}
	}
	s.jobs.update(jobID, func(job *Job) {
		job.Status = "done"
		job.Phase = "done"
		job.Message = "Render completo"
		job.Percent = 100
		job.Outputs = outputs
		job.FinishedAt = time.Now().UTC()
	})
}

func (s *Server) buildConfig(payload renderRequest) (pipeline.Config, error) {
	config := pipeline.Config{
		ClipSeconds:   payload.ClipSeconds,
		ClipCopies:    payload.ClipCopies,
		FPS:           payload.FPS,
		Width:         payload.Width,
		Height:        payload.Height,
		Voice:         payload.Voice,
		SynthName:     payload.TTS,
		SceneMode:     payload.SceneMode,
		TargetMinutes: payload.TargetMinutes,
		LLM:           cervantes.NewLLMWriterFromEnv(),
	}
	if config.ClipSeconds == 0 {
		config.ClipSeconds = 2.5
	}
	if config.ClipCopies == 0 {
		config.ClipCopies = 1
	}
	if config.FPS == 0 {
		config.FPS = 12
	}
	if config.Width == 0 {
		config.Width = 640
	}
	if config.Height == 0 {
		config.Height = 360
	}
	if config.Voice == "" {
		config.Voice = "default"
	}
	if config.SynthName == "" {
		config.SynthName = "tone"
	}

	if config.ClipSeconds < 0.25 || config.ClipSeconds > 30 {
		return pipeline.Config{}, errors.New("clip_seconds debe estar entre 0.25 y 30")
	}
	if config.ClipCopies < 1 || config.ClipCopies > 100 {
		return pipeline.Config{}, errors.New("clip_copies debe estar entre 1 y 100")
	}
	if config.FPS < 1 || config.FPS > 120 {
		return pipeline.Config{}, errors.New("fps debe estar entre 1 y 120")
	}
	if config.Width < 64 || config.Width > 3840 || config.Height < 64 || config.Height > 2160 {
		return pipeline.Config{}, errors.New("resolución fuera de rango (máx 3840x2160)")
	}

	switch strings.ToLower(payload.Renderer) {
	case "", "raster":
		config.Renderer = pixar.NewRaster()
	case "chromium":
		config.Renderer = pixar.NewChromium(payload.Chrome)
	default:
		return pipeline.Config{}, errors.New("renderer inválido")
	}
	switch strings.ToLower(payload.Encoder) {
	case "", "auto":
		config.Encoder = encoder.Select("auto")
	case "gif":
		config.Encoder = encoder.GIFEncoder{}
	case "ffmpeg":
		config.Encoder = encoder.Select("ffmpeg")
	default:
		return pipeline.Config{}, errors.New("encoder inválido")
	}
	switch strings.ToLower(config.SynthName) {
	case "tone":
		config.Synth = pavarotti.ToneSynth{}
	case "http":
		if strings.TrimSpace(payload.TTSURL) == "" {
			return pipeline.Config{}, errors.New("tts http requiere tts_url")
		}
		config.Synth = pavarotti.HTTPSynth{BaseURL: payload.TTSURL}
	default:
		return pipeline.Config{}, errors.New("tts inválido")
	}
	switch strings.ToLower(payload.Atlas) {
	case "", "deterministic":
		config.Atlas = atlas.DeterministicBuilder{}
	case "github":
		config.Atlas = atlas.GitHubBuilder{
			Repo:  payload.Repo,
			Ref:   payload.Ref,
			Token: os.Getenv("GITHUB_TOKEN"),
		}
	default:
		return pipeline.Config{}, errors.New("atlas inválido")
	}
	config.AllowReview = true
	return config, nil
}

func (s *Server) saveCase(engineeringCase contract.EngineeringCase) (string, error) {
	publicID := strings.TrimSuffix(store.FileName(engineeringCase.ID), ".json")
	path := filepath.Join(s.casesDir, publicID+".json")
	body, err := json.MarshalIndent(engineeringCase, "", "  ")
	if err != nil {
		return "", err
	}
	body = append(body, '\n')
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", err
	}
	return publicID, nil
}

func (s *Server) loadCase(publicID string) (contract.EngineeringCase, error) {
	if !safeID(publicID) {
		return contract.EngineeringCase{}, errors.New("id inválido")
	}
	body, err := os.ReadFile(filepath.Join(s.casesDir, publicID+".json"))
	if err != nil {
		return contract.EngineeringCase{}, err
	}
	var engineeringCase contract.EngineeringCase
	if err := json.Unmarshal(body, &engineeringCase); err != nil {
		return contract.EngineeringCase{}, err
	}
	if err := engineeringCase.Validate(); err != nil {
		return contract.EngineeringCase{}, err
	}
	return engineeringCase, nil
}

func (s *Server) listCases() ([]caseSummary, error) {
	entries, err := os.ReadDir(s.casesDir)
	if err != nil {
		return nil, err
	}
	cases := make([]caseSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		publicID := strings.TrimSuffix(entry.Name(), ".json")
		engineeringCase, err := s.loadCase(publicID)
		if err != nil {
			continue
		}
		cases = append(cases, caseSummary{
			ID:          publicID,
			CaseID:      engineeringCase.ID,
			Title:       engineeringCase.Title,
			Verdict:     engineeringCase.HypeFilter.Verdict,
			Score:       engineeringCase.HypeFilter.Score,
			Review:      s.readReview(publicID).Decision,
			GeneratedAt: engineeringCase.GeneratedAt,
			Runs:        len(s.scanRuns(engineeringCase.ID)),
		})
	}
	sort.Slice(cases, func(i, j int) bool {
		return cases[i].GeneratedAt.After(cases[j].GeneratedAt)
	})
	return cases, nil
}

func (s *Server) readReview(publicID string) reviewRecord {
	body, err := os.ReadFile(filepath.Join(s.reviewsDir, publicID+".json"))
	if err != nil {
		return reviewRecord{Decision: "pending"}
	}
	var record reviewRecord
	if json.Unmarshal(body, &record) != nil || record.Decision == "" {
		return reviewRecord{Decision: "pending"}
	}
	return record
}

func (s *Server) writeReview(publicID string, record reviewRecord) error {
	body, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(filepath.Join(s.reviewsDir, publicID+".json"), body, 0o644)
}

func (s *Server) scanRuns(caseID string) []runSummary {
	entries, err := os.ReadDir(s.runsDir)
	if err != nil {
		return nil
	}
	runs := make([]runSummary, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		body, err := os.ReadFile(filepath.Join(s.runsDir, entry.Name(), "run_manifest.json"))
		if err != nil {
			continue
		}
		var manifest contract.RunManifest
		if json.Unmarshal(body, &manifest) != nil || manifest.CaseID != caseID {
			continue
		}
		outputs := make([]JobOutput, 0, len(manifest.Outputs))
		for _, output := range manifest.Outputs {
			if relative := s.relPath(output.Path); relative != "" {
				outputs = append(outputs, JobOutput{Kind: output.Kind, Path: relative})
			}
		}
		clips := make([]runClip, 0, len(manifest.Clips))
		for _, clip := range manifest.Clips {
			clips = append(clips, runClip{
				ID:     clip.ID,
				HTML:   s.relPath(clip.HTML),
				Audio:  s.relPath(clip.Audio),
				Frames: clip.Frames,
			})
		}
		runs = append(runs, runSummary{
			ID:         entry.Name(),
			FinishedAt: manifest.FinishedAt,
			Frames:     manifest.FrameCount,
			Outputs:    outputs,
			Clips:      clips,
		})
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].FinishedAt.After(runs[j].FinishedAt)
	})
	return runs
}

func (s *Server) relPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	relative, err := filepath.Rel(s.workspace, absolute)
	if err != nil || relative == "." || strings.HasPrefix(relative, "..") {
		return ""
	}
	return filepath.ToSlash(relative)
}

func (s *Server) seed() {
	if cases, _ := s.listCases(); len(cases) > 0 {
		return
	}
	matches, _ := filepath.Glob(filepath.Join("out", "final", "cases", "*.json"))
	for _, match := range matches {
		body, err := os.ReadFile(match)
		if err != nil {
			continue
		}
		var engineeringCase contract.EngineeringCase
		if json.Unmarshal(body, &engineeringCase) == nil && engineeringCase.Validate() == nil {
			_, _ = s.saveCase(engineeringCase)
		}
	}
	if cases, _ := s.listCases(); len(cases) > 0 {
		return
	}

	fixture := filepath.Join("engine", "internal", "galileo", "testdata", "postmortem.md")
	document, err := source.FromFile(fixture, source.Options{})
	if err != nil {
		body, readErr := staticFiles.ReadFile("seed/postmortem.md")
		if readErr != nil {
			return
		}
		document = source.FromText(string(body), source.Options{Type: "postmortem"})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := galileo.New(extract.MarkedExtractor{}, filepath.Join(s.workspace, "seeds")).Run(ctx, document)
	if err != nil {
		return
	}
	_, _ = s.saveCase(result.Case)
}

func safeID(id string) bool {
	if id == "" || id != filepath.Base(id) {
		return false
	}
	if id == "." || id == ".." || strings.ContainsAny(id, `/\:`) {
		return false
	}
	return true
}

func randomHex(bytes int) string {
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

func decodeJSON(request *http.Request, target any) error {
	body, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return errors.New("cuerpo vacío")
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("JSON inválido: %w", err)
	}
	return nil
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]string{"error": message})
}
