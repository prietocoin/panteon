package source

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"elpanteon/engine/internal/galileo/contract"
)

// MaxBodyBytes limita el tamaño de una fuente descargada.
const MaxBodyBytes = 5 << 20

// Document es una fuente primaria ya normalizada.
type Document struct {
	URL         string
	Type        string
	Title       string
	Text        string
	RetrievedAt time.Time
	ContentHash string
}

// Options permite sobrescribir metadatos inferidos.
type Options struct {
	URL         string
	Type        string
	Title       string
	RetrievedAt time.Time
}

var (
	markdownTitleRe = regexp.MustCompile(`(?m)^#{1,6}[ \t]+(.+?)[ \t]*$`)
	spaceRe         = regexp.MustCompile(`[ \t]+`)
)

// FromText normaliza el contenido y construye un Document.
func FromText(raw string, opts Options) Document {
	text := NormalizeText(raw)
	doc := Document{
		URL:         opts.URL,
		Type:        opts.Type,
		Title:       opts.Title,
		Text:        text,
		RetrievedAt: opts.RetrievedAt,
		ContentHash: contract.ContentHash(text),
	}
	if doc.Type == "" {
		doc.Type = "other"
	}
	if doc.RetrievedAt.IsZero() {
		doc.RetrievedAt = time.Now().UTC()
	}
	if doc.Title == "" {
		doc.Title = firstMarkdownTitle(text)
	}
	return doc
}

// FromFile lee una fuente local (.md, .txt, .html) y la normaliza.
func FromFile(path string, opts Options) (Document, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Document{}, fmt.Errorf("leyendo %s: %w", path, err)
	}

	raw := string(body)
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".html" || ext == ".htm" {
		text, title := HTMLToText(raw)
		raw = text
		if opts.Title == "" {
			opts.Title = title
		}
	}
	if opts.Type == "" {
		opts.Type = InferType(path)
	}

	doc := FromText(raw, opts)
	if doc.Title == "" {
		doc.Title = strings.TrimSuffix(filepath.Base(path), ext)
	}
	return doc, nil
}

// FromURL descarga una fuente pública y la normaliza.
func FromURL(ctx context.Context, rawURL string, client *http.Client) (Document, error) {
	if err := ValidatePublicURL(ctx, rawURL); err != nil {
		return Document{}, err
	}
	if client == nil {
		client = defaultClient()
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Document{}, fmt.Errorf("creando petición: %w", err)
	}
	request.Header.Set("User-Agent", "el-panteon-galileo/0.1 (+https://example.invalid/galileo)")

	response, err := client.Do(request)
	if err != nil {
		return Document{}, fmt.Errorf("descargando fuente: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return Document{}, fmt.Errorf("la fuente respondió %s", response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, MaxBodyBytes+1))
	if err != nil {
		return Document{}, fmt.Errorf("leyendo respuesta: %w", err)
	}
	if len(body) > MaxBodyBytes {
		return Document{}, fmt.Errorf("la fuente supera el límite de %d bytes", MaxBodyBytes)
	}

	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	raw := string(body)
	title := ""
	if strings.Contains(contentType, "html") || strings.HasPrefix(strings.TrimSpace(raw), "<") {
		raw, title = HTMLToText(raw)
	}

	return FromText(raw, Options{
		URL:   rawURL,
		Type:  InferType(rawURL),
		Title: title,
	}), nil
}

// ValidatePublicURL bloquea esquemas no HTTP y destinos internos (SSRF).
func ValidatePublicURL(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("URL inválida: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("esquema no permitido: %q", parsed.Scheme)
	}
	host := parsed.Hostname()
	if host == "" {
		return errors.New("la URL no tiene host")
	}

	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolviendo %s: %w", host, err)
	}
	for _, address := range addresses {
		if isBlockedIP(address.IP) {
			return fmt.Errorf("destino bloqueado: %s resuelve a %s", host, address.IP)
		}
	}
	return nil
}

func defaultClient() *http.Client {
	return &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("demasiadas redirecciones")
			}
			return ValidatePublicURL(request.Context(), request.URL.String())
		},
	}
}

func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast()
}

// NormalizeText unifica saltos de línea y colapsa líneas en blanco.
func NormalizeText(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")

	lines := make([]string, 0, 128)
	previousBlank := false
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			if !previousBlank && len(lines) > 0 {
				lines = append(lines, "")
			}
			previousBlank = true
			continue
		}
		previousBlank = false
		lines = append(lines, line)
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// InferType deduce el tipo de fuente a partir de la ruta o URL.
func InferType(value string) string {
	lower := strings.ToLower(value)
	switch {
	case strings.Contains(lower, "postmortem"), strings.Contains(lower, "post-mortem"), strings.Contains(lower, "incident"):
		return "postmortem"
	case strings.Contains(lower, "rfc"):
		return "rfc"
	case strings.Contains(lower, "adr"):
		return "adr"
	default:
		return "other"
	}
}

func firstMarkdownTitle(text string) string {
	if match := markdownTitleRe.FindStringSubmatch(text); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

var (
	scriptStyleRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>|<noscript[^>]*>.*?</noscript>`)
	titleTagRe    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	breakTagRe    = regexp.MustCompile(`(?i)<br\s*/?>`)
	blockTagRe    = regexp.MustCompile(`(?i)</?(p|div|section|article|header|footer|h[1-6]|li|tr|table|ul|ol|blockquote|pre)[^>]*>`)
	anyTagRe      = regexp.MustCompile(`(?s)<[^>]+>`)
)

// HTMLToText extrae texto legible y el título de un documento HTML.
func HTMLToText(raw string) (string, string) {
	title := ""
	if match := titleTagRe.FindStringSubmatch(raw); len(match) > 1 {
		title = strings.TrimSpace(html.UnescapeString(anyTagRe.ReplaceAllString(match[1], " ")))
	}

	text := scriptStyleRe.ReplaceAllString(raw, " ")
	text = breakTagRe.ReplaceAllString(text, "\n")
	text = blockTagRe.ReplaceAllString(text, "\n")
	text = anyTagRe.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(spaceRe.ReplaceAllString(line, " "))
	}
	return NormalizeText(strings.Join(lines, "\n")), title
}
