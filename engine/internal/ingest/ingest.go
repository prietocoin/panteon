package ingest

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"elpanteon/engine/internal/galileo"
	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/galileo/extract"
	"elpanteon/engine/internal/galileo/source"
)

const (
	defaultIntervalMinutes = 60
	maxItemsPerSource      = 12
	maxBodyBytes           = 5 << 20
	maxEvents              = 200
)

var defaultKeywords = []string{
	"postmortem", "post-mortem", "incident", "outage", "root cause",
	"failure", "fallo", "incidente", "caída", "caida", "retrospective",
	"blameless", "what went wrong",
}

// Source es una fuente de ingesta automática.
type Source struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"` // rss | github
	Name            string    `json:"name"`
	URL             string    `json:"url,omitempty"`
	Repo            string    `json:"repo,omitempty"`
	IntervalMinutes int       `json:"interval_minutes"`
	Enabled         bool      `json:"enabled"`
	AutoCreate      bool      `json:"auto_create"`
	Keywords        []string  `json:"keywords,omitempty"`
	LastCheckedAt   time.Time `json:"last_checked_at,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// Event registra el resultado de una ingesta.
type Event struct {
	Time       time.Time `json:"time"`
	SourceID   string    `json:"source_id"`
	SourceName string    `json:"source_name"`
	Title      string    `json:"title"`
	URL        string    `json:"url,omitempty"`
	Status     string    `json:"status"` // created | skipped | error
	Message    string    `json:"message,omitempty"`
	CaseID     string    `json:"case_id,omitempty"`
}

// Item es una entrada normalizada de RSS o GitHub.
type Item struct {
	ID        string
	Title     string
	URL       string
	Content   string
	Published time.Time
}

// Notifier envía avisos fuera de la UI.
type Notifier interface {
	Notify(ctx context.Context, event Event, engineeringCase *contract.EngineeringCase) error
}

// Service orquesta fuentes, deduplicación y ejecución de Galileo.
type Service struct {
	mu        sync.Mutex
	dir       string
	sources   []Source
	seen      map[string]time.Time
	events    []Event
	running   bool
	client    *http.Client
	extractor extract.Extractor
	saveCase  func(contract.EngineeringCase) (string, error)
	notifier  Notifier
}

// New carga o crea el estado de ingesta.
func New(dir string, extractor extract.Extractor, saveCase func(contract.EngineeringCase) (string, error), notifier Notifier) (*Service, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	service := &Service{
		dir:     dir,
		sources: make([]Source, 0),
		seen:    make(map[string]time.Time),
		events:  make([]Event, 0),
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(request *http.Request, via []*http.Request) error {
				return source.ValidatePublicURL(request.Context(), request.URL.String())
			},
		},
		extractor: extractor,
		saveCase:  saveCase,
		notifier:  notifier,
	}
	service.load()
	return service, nil
}

// Start arranca el watcher en background (cada minuto comprueba fuentes vencidas).
func (s *Service) Start(ctx context.Context) {
	go func() {
		_, _ = s.RunOnce(ctx, false)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = s.RunOnce(ctx, false)
			}
		}
	}()
}

// ListSources devuelve una copia de las fuentes.
func (s *Service) ListSources() []Source {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Source, len(s.sources))
	copy(out, s.sources)
	return out
}

// Events devuelve los eventos más recientes.
func (s *Service) Events() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// AddSource valida y guarda una fuente.
func (s *Service) AddSource(input Source) (Source, error) {
	input.Type = strings.ToLower(strings.TrimSpace(input.Type))
	if input.Type != "rss" && input.Type != "github" {
		return Source{}, errors.New("tipo de fuente inválido: usa rss o github")
	}
	if input.Type == "rss" && strings.TrimSpace(input.URL) == "" {
		return Source{}, errors.New("rss requiere url")
	}
	if input.Type == "github" && strings.TrimSpace(input.Repo) == "" {
		return Source{}, errors.New("github requiere repo owner/name")
	}
	if input.IntervalMinutes < 5 {
		input.IntervalMinutes = defaultIntervalMinutes
	}
	if input.ID == "" {
		input.ID = "src-" + randomHex(4)
	}
	if strings.TrimSpace(input.Name) == "" {
		input.Name = input.URL
		if input.Type == "github" {
			input.Name = input.Repo
		}
	}
	input.CreatedAt = time.Now().UTC()
	input.LastError = ""
	s.mu.Lock()
	s.sources = append(s.sources, input)
	s.saveSourcesLocked()
	s.mu.Unlock()
	return input, nil
}

// DeleteSource elimina una fuente por ID.
func (s *Service) DeleteSource(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, item := range s.sources {
		if item.ID == id {
			s.sources = append(s.sources[:index], s.sources[index+1:]...)
			s.saveSourcesLocked()
			return true
		}
	}
	return false
}

// RunOnce comprueba las fuentes vencidas (o todas si force=true).
func (s *Service) RunOnce(ctx context.Context, force bool) ([]Event, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil, errors.New("ingesta ya en curso")
	}
	s.running = true
	sources := make([]Source, len(s.sources))
	copy(sources, s.sources)
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	now := time.Now().UTC()
	var produced []Event
	for _, item := range sources {
		if !item.Enabled {
			continue
		}
		interval := time.Duration(item.IntervalMinutes) * time.Minute
		if interval <= 0 {
			interval = time.Duration(defaultIntervalMinutes) * time.Minute
		}
		if !force && !item.LastCheckedAt.IsZero() && now.Sub(item.LastCheckedAt) < interval {
			continue
		}

		items, err := s.fetchSource(ctx, item)
		s.mu.Lock()
		for index := range s.sources {
			if s.sources[index].ID != item.ID {
				continue
			}
			s.sources[index].LastCheckedAt = time.Now().UTC()
			if err != nil {
				s.sources[index].LastError = err.Error()
			} else {
				s.sources[index].LastError = ""
			}
		}
		s.saveSourcesLocked()
		s.mu.Unlock()
		if err != nil {
			event := Event{
				Time:       time.Now().UTC(),
				SourceID:   item.ID,
				SourceName: item.Name,
				Title:      "Error consultando la fuente",
				Status:     "error",
				Message:    err.Error(),
			}
			s.appendEvent(event)
			produced = append(produced, event)
			continue
		}

		for _, entry := range items {
			event := s.processItem(ctx, item, entry)
			produced = append(produced, event)
		}
	}
	return produced, nil
}

func (s *Service) processItem(ctx context.Context, sourceItem Source, item Item) Event {
	event := Event{
		Time:       time.Now().UTC(),
		SourceID:   sourceItem.ID,
		SourceName: sourceItem.Name,
		Title:      item.Title,
		URL:        item.URL,
	}

	key := itemKey(item)
	s.mu.Lock()
	_, alreadySeen := s.seen[key]
	s.mu.Unlock()
	if alreadySeen {
		event.Status = "skipped"
		event.Message = "ya procesado"
		return event
	}

	s.mu.Lock()
	s.seen[key] = time.Now().UTC()
	s.saveSeenLocked()
	s.mu.Unlock()

	content := item.Content
	title := item.Title
	if len(content) < 1200 && strings.HasPrefix(item.URL, "http") {
		if document, err := source.FromURL(ctx, item.URL, nil); err == nil && len(document.Text) > len(content) {
			content = document.Text
			if strings.TrimSpace(document.Title) != "" {
				title = document.Title
			}
		}
	}
	enriched := Item{ID: item.ID, Title: title, URL: item.URL, Content: content, Published: item.Published}
	if !matchesKeywords(sourceItem, enriched) {
		event.Status = "skipped"
		event.Message = "no coincide con las palabras clave"
		s.appendEvent(event)
		return event
	}
	if !sourceItem.AutoCreate {
		event.Status = "skipped"
		event.Message = "auto_create desactivado"
		s.appendEvent(event)
		return event
	}

	document := source.FromText(content, source.Options{
		URL:         item.URL,
		Title:       title,
		Type:        "other",
		RetrievedAt: item.Published,
	})
	runCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	result, err := galileo.New(s.extractor, filepath.Join(s.dir, "seeds")).Run(runCtx, document)
	if err != nil {
		event.Status = "error"
		event.Message = err.Error()
		s.appendEvent(event)
		return event
	}
	caseID, err := s.saveCase(result.Case)
	if err != nil {
		event.Status = "error"
		event.Message = err.Error()
		s.appendEvent(event)
		return event
	}
	event.Status = "created"
	event.CaseID = caseID
	event.Message = "caso creado"
	s.appendEvent(event)
	if s.notifier != nil {
		_ = s.notifier.Notify(ctx, event, &result.Case)
	}
	return event
}

func (s *Service) fetchSource(ctx context.Context, item Source) ([]Item, error) {
	switch item.Type {
	case "rss":
		return s.fetchRSS(ctx, item)
	case "github":
		return s.fetchGitHub(ctx, item)
	default:
		return nil, fmt.Errorf("tipo desconocido: %s", item.Type)
	}
}

func (s *Service) fetchRSS(ctx context.Context, item Source) ([]Item, error) {
	body, err := s.fetchURL(ctx, item.URL, "")
	if err != nil {
		return nil, err
	}
	items := parseFeed(body)
	if len(items) > maxItemsPerSource {
		items = items[:maxItemsPerSource]
	}
	return items, nil
}

type githubIssue struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	State       string    `json:"state"`
	UpdatedAt   string    `json:"updated_at"`
	PullRequest *struct{} `json:"pull_request"`
}

func (s *Service) fetchGitHub(ctx context.Context, item Source) ([]Item, error) {
	repo := strings.Trim(strings.TrimSpace(item.Repo), "/")
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/issues?state=all&sort=updated&per_page=30", repo)
	body, err := s.fetchURL(ctx, endpoint, os.Getenv("GITHUB_TOKEN"))
	if err != nil {
		return nil, err
	}
	var issues []githubIssue
	if err := json.Unmarshal(body, &issues); err != nil {
		return nil, fmt.Errorf("GitHub JSON: %w", err)
	}
	items := make([]Item, 0, len(issues))
	for _, issue := range issues {
		if issue.PullRequest != nil {
			continue
		}
		published, _ := time.Parse(time.RFC3339, issue.UpdatedAt)
		items = append(items, Item{
			ID:        fmt.Sprintf("github:%s:%d:%s", repo, issue.Number, issue.UpdatedAt),
			Title:     issue.Title,
			URL:       issue.HTMLURL,
			Content:   issue.Title + "\n\n" + issue.Body,
			Published: published,
		})
	}
	if len(items) > maxItemsPerSource {
		items = items[:maxItemsPerSource]
	}
	return items, nil
}

func (s *Service) fetchURL(ctx context.Context, rawURL, token string) ([]byte, error) {
	if err := source.ValidatePublicURL(ctx, rawURL); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "el-panteon-ingest/0.1")
	request.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/json, text/xml, */*")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("fuente respondió %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}
	return body, nil
}

func matchesKeywords(sourceItem Source, item Item) bool {
	text := strings.ToLower(item.Title + " " + item.Content)
	keywords := sourceItem.Keywords
	if len(keywords) == 0 {
		keywords = defaultKeywords
	}
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func itemKey(item Item) string {
	sum := sha256.Sum256([]byte(item.ID + "|" + item.URL + "|" + item.Content))
	return hex.EncodeToString(sum[:])
}

func (s *Service) appendEvent(event Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	if len(s.events) > maxEvents {
		s.events = s.events[len(s.events)-maxEvents:]
	}
	s.saveEventsLocked()
}

func (s *Service) load() {
	_ = readJSON(filepath.Join(s.dir, "sources.json"), &s.sources)
	_ = readJSON(filepath.Join(s.dir, "seen.json"), &s.seen)
	_ = readJSON(filepath.Join(s.dir, "events.json"), &s.events)
	if s.seen == nil {
		s.seen = make(map[string]time.Time)
	}
}

func (s *Service) saveSourcesLocked() {
	_ = writeJSON(filepath.Join(s.dir, "sources.json"), s.sources)
}

func (s *Service) saveSeenLocked() {
	_ = writeJSON(filepath.Join(s.dir, "seen.json"), s.seen)
}

func (s *Service) saveEventsLocked() {
	_ = writeJSON(filepath.Join(s.dir, "events.json"), s.events)
}

func writeJSON(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}

func readJSON(path string, target any) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}

func randomHex(n int) string {
	buffer := make([]byte, n)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

// parseFeed soporta RSS 2.0 y Atom sin dependencias externas.
func parseFeed(body []byte) []Item {
	text := strings.ToLower(string(body))
	if strings.Contains(text, "<feed") {
		var feed atomDocument
		if xml.Unmarshal(body, &feed) == nil && len(feed.Entries) > 0 {
			items := make([]Item, 0, len(feed.Entries))
			for _, entry := range feed.Entries {
				content := sourceText(entry.Content, entry.Summary)
				link := ""
				if len(entry.Links) > 0 {
					link = entry.Links[0].Href
				}
				items = append(items, Item{
					ID:        firstNonEmpty(link, entry.ID, entry.Title),
					Title:     strings.TrimSpace(entry.Title),
					URL:       link,
					Content:   content,
					Published: parseTime(firstNonEmpty(entry.Published, entry.Updated)),
				})
			}
			return items
		}
	}
	var feed rssDocument
	if xml.Unmarshal(body, &feed) != nil {
		return nil
	}
	items := make([]Item, 0, len(feed.Channel.Items))
	for _, entry := range feed.Channel.Items {
		content := sourceText(entry.Content, entry.Description)
		items = append(items, Item{
			ID:        firstNonEmpty(entry.Link, entry.GUID, entry.Title),
			Title:     strings.TrimSpace(entry.Title),
			URL:       entry.Link,
			Content:   content,
			Published: parseTime(entry.PubDate),
		})
	}
	return items
}

type rssDocument struct {
	Channel struct {
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			GUID        string `xml:"guid"`
			Description string `xml:"description"`
			Content     string `xml:"encoded"`
			PubDate     string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
}

type atomDocument struct {
	Entries []struct {
		ID        string `xml:"id"`
		Title     string `xml:"title"`
		Summary   string `xml:"summary"`
		Content   string `xml:"content"`
		Published string `xml:"published"`
		Updated   string `xml:"updated"`
		Links     []struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
	} `xml:"entry"`
}

func sourceText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		text, _ := source.HTMLToText(value)
		if strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func parseTime(value string) time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339, time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// NotifyFromEnv crea el notificador configurado por variables de entorno.
func NotifyFromEnv() Notifier {
	notifiers := make([]Notifier, 0, 2)
	if url := strings.TrimSpace(os.Getenv("PANTEON_NOTIFY_WEBHOOK")); url != "" {
		notifiers = append(notifiers, WebhookNotifier{URL: url})
	}
	token := strings.TrimSpace(os.Getenv("PANTEON_TELEGRAM_TOKEN"))
	chatID := strings.TrimSpace(os.Getenv("PANTEON_TELEGRAM_CHAT_ID"))
	if token != "" && chatID != "" {
		notifiers = append(notifiers, TelegramNotifier{Token: token, ChatID: chatID})
	}
	if len(notifiers) == 0 {
		return nil
	}
	return MultiNotifier(notifiers)
}

// WebhookNotifier envía a Discord/Slack/genérico.
type WebhookNotifier struct {
	URL string
}

func (w WebhookNotifier) Notify(ctx context.Context, event Event, engineeringCase *contract.EngineeringCase) error {
	if engineeringCase == nil {
		return nil
	}
	message := fmt.Sprintf("🏛️ Nuevo caso: %s\nVeredicto: %s (%d/100)\n%s",
		engineeringCase.Title, engineeringCase.HypeFilter.Verdict, engineeringCase.HypeFilter.Score, event.URL)
	payload := map[string]string{"content": message, "text": message}
	body, _ := json.Marshal(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("webhook respondió %s", response.Status)
	}
	return nil
}

// TelegramNotifier envía un mensaje a Telegram.
type TelegramNotifier struct {
	Token  string
	ChatID string
}

func (t TelegramNotifier) Notify(ctx context.Context, event Event, engineeringCase *contract.EngineeringCase) error {
	if engineeringCase == nil {
		return nil
	}
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.Token)
	message := fmt.Sprintf("🏛️ Nuevo caso: %s\nVeredicto: %s (%d/100)\n%s",
		engineeringCase.Title, engineeringCase.HypeFilter.Verdict, engineeringCase.HypeFilter.Score, event.URL)
	payload, _ := json.Marshal(map[string]any{"chat_id": t.ChatID, "text": message, "disable_web_page_preview": false})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("telegram respondió %s", response.Status)
	}
	return nil
}

// MultiNotifier ejecuta varios notificadores.
type MultiNotifier []Notifier

func (m MultiNotifier) Notify(ctx context.Context, event Event, engineeringCase *contract.EngineeringCase) error {
	var first error
	for _, notifier := range m {
		if err := notifier.Notify(ctx, event, engineeringCase); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// SortSources ordena por creación descendente para la UI.
func SortSources(sources []Source) {
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].CreatedAt.After(sources[j].CreatedAt)
	})
}
