package atlas

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/textutil"
)

const (
	defaultAPIBase = "https://api.github.com"
	defaultRawBase = "https://raw.githubusercontent.com"
	maxJSONBytes   = 16 << 20
	maxFileBytes   = 8 << 10
	maxReadmeBytes = 12 << 10
)

// GitHubBuilder construye el dossier con metadatos, README, código e issues reales.
// Si Repo está vacío se infiere de source.url cuando es un enlace de github.com.
type GitHubBuilder struct {
	Repo     string
	Ref      string
	Token    string
	APIBase  string
	RawBase  string
	Client   *http.Client
	MaxFiles int
	Now      func() time.Time
}

type githubRepository struct {
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	Language      string `json:"language"`
	Stars         int    `json:"stargazers_count"`
	DefaultBranch string `json:"default_branch"`
	HTMLURL       string `json:"html_url"`
}

type githubTree struct {
	Tree []githubTreeEntry `json:"tree"`
}

type githubTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int    `json:"size"`
}

type githubIssue struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	State       string    `json:"state"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	PullRequest *struct{} `json:"pull_request"`
}

// Build implementa Builder.
func (b GitHubBuilder) Build(ctx context.Context, engineeringCase contract.EngineeringCase) (contract.Dossier, error) {
	if err := engineeringCase.Validate(); err != nil {
		return contract.Dossier{}, fmt.Errorf("atlas/github: %w", err)
	}
	repo := strings.TrimSpace(b.Repo)
	if repo == "" {
		repo = InferRepo(engineeringCase.Source.URL)
	}
	if repo == "" {
		return contract.Dossier{}, fmt.Errorf("atlas/github: indica un repositorio owner/name o una URL de GitHub")
	}

	repository, err := b.repository(ctx, repo)
	if err != nil {
		return contract.Dossier{}, err
	}
	ref := strings.TrimSpace(b.Ref)
	if ref == "" {
		ref = repository.DefaultBranch
	}
	if ref == "" {
		ref = "main"
	}

	tree, err := b.tree(ctx, repo, ref)
	if err != nil {
		return contract.Dossier{}, err
	}
	files := selectCodeFiles(tree, b.maxFiles())

	readme, _ := b.rawFile(ctx, repo, ref, "README.md", maxReadmeBytes)
	code := b.codeArtifact(ctx, repo, ref, files)
	issues := b.issuesArtifact(ctx, repo)

	now := time.Now
	if b.Now != nil {
		now = b.Now
	}

	source := repository.HTMLURL
	if source == "" {
		source = "https://github.com/" + repo
	}
	dossier := contract.Dossier{
		SchemaVersion: contract.DossierSchemaVersion,
		CaseID:        engineeringCase.ID,
		Title:         engineeringCase.Title,
		Summary: fmt.Sprintf(
			"%s Repositorio %s (%s, %d estrellas); %d archivos de código y %s.",
			textutil.Clean(engineeringCase.Symptom.Summary),
			repository.FullName,
			orDefault(repository.Language, "lenguaje desconocido"),
			repository.Stars,
			len(files),
			b.refLabel(ref),
		),
		Facts:       githubFacts(engineeringCase, repository, ref, len(files), source),
		Artifacts:   githubArtifacts(engineeringCase, repository, ref, readme, code, issues, source),
		GeneratedAt: now().UTC(),
	}

	if err := dossier.Validate(); err != nil {
		return contract.Dossier{}, fmt.Errorf("atlas/github: %w", err)
	}
	return dossier, nil
}

func (b GitHubBuilder) repository(ctx context.Context, repo string) (githubRepository, error) {
	var repository githubRepository
	err := b.getJSON(ctx, b.apiBase()+"/repos/"+escapeRepoSegments(repo), &repository)
	if err != nil {
		return githubRepository{}, fmt.Errorf("atlas/github: %w", err)
	}
	return repository, nil
}

func (b GitHubBuilder) tree(ctx context.Context, repo, ref string) (githubTree, error) {
	var tree githubTree
	endpoint := b.apiBase() + "/repos/" + escapeRepoSegments(repo) + "/git/trees/" + url.PathEscape(ref) + "?recursive=1"
	if err := b.getJSON(ctx, endpoint, &tree); err != nil {
		return githubTree{}, fmt.Errorf("atlas/github: %w", err)
	}
	return tree, nil
}

func (b GitHubBuilder) codeArtifact(ctx context.Context, repo, ref string, files []githubTreeEntry) string {
	if len(files) == 0 {
		return "No se encontraron archivos de código compatibles."
	}
	var builder strings.Builder
	for _, file := range files {
		body, err := b.rawFile(ctx, repo, ref, file.Path, maxFileBytes)
		if err != nil || strings.TrimSpace(body) == "" {
			continue
		}
		builder.WriteString("// ")
		builder.WriteString(file.Path)
		builder.WriteString("\n")
		builder.WriteString(strings.TrimSpace(body))
		builder.WriteString("\n\n")
	}
	if builder.Len() == 0 {
		return "No se pudo descargar el contenido de los archivos."
	}
	return strings.TrimSpace(builder.String())
}

func (b GitHubBuilder) issuesArtifact(ctx context.Context, repo string) string {
	var issues []githubIssue
	endpoint := b.apiBase() + "/repos/" + escapeRepoSegments(repo) + "/issues?state=all&sort=updated&per_page=10"
	if err := b.getJSON(ctx, endpoint, &issues); err != nil {
		return "No se pudieron consultar los issues del repositorio."
	}
	var builder strings.Builder
	count := 0
	for _, issue := range issues {
		if issue.PullRequest != nil {
			continue
		}
		fmt.Fprintf(&builder, "#%d [%s] %s\n", issue.Number, issue.State, issue.Title)
		if body := textutil.Truncate(textutil.Clean(issue.Body), 240); body != "" {
			builder.WriteString("  ")
			builder.WriteString(body)
			builder.WriteString("\n")
		}
		count++
		if count == 5 {
			break
		}
	}
	if count == 0 {
		return "No se encontraron issues abiertos o cerrados."
	}
	return strings.TrimSpace(builder.String())
}

func (b GitHubBuilder) rawFile(ctx context.Context, repo, ref, filePath string, limit int64) (string, error) {
	endpoint := b.rawBase() + "/" + escapeRepoSegments(repo) + "/" + url.PathEscape(ref) + "/" + escapePath(filePath)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "el-panteon-atlas/0.1")
	response, err := b.client().Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github raw %s: %s", endpoint, response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(body)) > limit {
		body = body[:limit]
	}
	return string(body), nil
}

func (b GitHubBuilder) getJSON(ctx context.Context, endpoint string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "el-panteon-atlas/0.1")
	if token := strings.TrimSpace(b.Token); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := b.client().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("GitHub respondió %s en %s", response.Status, endpoint)
	}
	return json.NewDecoder(io.LimitReader(response.Body, maxJSONBytes)).Decode(target)
}

func (b GitHubBuilder) client() *http.Client {
	if b.Client != nil {
		return b.Client
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (b GitHubBuilder) apiBase() string {
	if strings.TrimSpace(b.APIBase) != "" {
		return strings.TrimRight(b.APIBase, "/")
	}
	return defaultAPIBase
}

func (b GitHubBuilder) rawBase() string {
	if strings.TrimSpace(b.RawBase) != "" {
		return strings.TrimRight(b.RawBase, "/")
	}
	return defaultRawBase
}

func (b GitHubBuilder) maxFiles() int {
	if b.MaxFiles <= 0 {
		return 3
	}
	if b.MaxFiles > 10 {
		return 10
	}
	return b.MaxFiles
}

func (b GitHubBuilder) refLabel(ref string) string {
	return "ref " + ref
}

// InferRepo extrae owner/name de una URL de GitHub.
func InferRepo(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if !strings.EqualFold(parsed.Hostname(), "github.com") && !strings.EqualFold(parsed.Hostname(), "www.github.com") {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

func githubFacts(engineeringCase contract.EngineeringCase, repository githubRepository, ref string, fileCount int, source string) []contract.Fact {
	verification := engineeringCase.Solution.Verification
	if strings.TrimSpace(verification) == "" {
		verification = "No declarada en la fuente."
	}
	tradeoffs := strings.Join(engineeringCase.Solution.Tradeoffs, " ")
	if strings.TrimSpace(tradeoffs) == "" {
		tradeoffs = "No declaradas en la fuente."
	}
	return []contract.Fact{
		{Label: "Repositorio", Value: repository.FullName, Source: source},
		{Label: "Descripción", Value: orDefault(repository.Description, "Sin descripción"), Source: source},
		{Label: "Lenguaje principal", Value: orDefault(repository.Language, "Desconocido"), Source: source},
		{Label: "Estrellas", Value: fmt.Sprintf("%d", repository.Stars), Source: source},
		{Label: "Ref analizada", Value: ref, Source: source},
		{Label: "Archivos de código", Value: fmt.Sprintf("%d", fileCount), Source: source},
		{Label: "Síntoma", Value: engineeringCase.Symptom.Summary, Source: "engineering_case"},
		{Label: "Causa", Value: engineeringCase.Cause.Summary, Source: "engineering_case"},
		{Label: "Solución", Value: engineeringCase.Solution.Summary, Source: "engineering_case"},
		{Label: "Verificación", Value: verification, Source: "engineering_case"},
		{Label: "Trade-offs", Value: tradeoffs, Source: "engineering_case"},
	}
}

func githubArtifacts(
	engineeringCase contract.EngineeringCase,
	repository githubRepository,
	ref, readme, code, issues, source string,
) []contract.Artifact {
	readmeContent := strings.TrimSpace(readme)
	if readmeContent == "" {
		readmeContent = "README no disponible."
	}
	cause := strings.TrimSpace(engineeringCase.Cause.Mechanism)
	if cause == "" {
		cause = engineeringCase.Cause.Summary
	}
	verification := engineeringCase.Solution.Verification
	if strings.TrimSpace(verification) == "" {
		verification = "No declarada en la fuente."
	}
	return []contract.Artifact{
		{ID: "symptom-evidence", Kind: "benchmark", Title: "Evidencia del síntoma", Content: quotes(engineeringCase.Symptom.Evidence), Source: source},
		{ID: "cause-mechanism", Kind: "diagram", Title: "Cadena causal", Content: cause, Source: source},
		{ID: "solution-code", Kind: "code", Title: "Plan de remediación", Content: remediationCode(engineeringCase), Source: source},
		{ID: "verification", Kind: "reference", Title: "Verificación", Content: verification, Source: source},
		{ID: "repo-readme", Kind: "reference", Title: "README de " + repository.FullName, Content: readmeContent, Source: source + "/blob/" + ref + "/README.md"},
		{ID: "repo-code", Kind: "code", Title: "Código real del repositorio", Content: code, Source: source + "/tree/" + ref},
		{ID: "repo-issues", Kind: "benchmark", Title: "Issues recientes", Content: issues, Source: source + "/issues"},
	}
}

func selectCodeFiles(tree githubTree, max int) []githubTreeEntry {
	extensions := map[string]bool{
		".go": true, ".py": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".rs": true, ".java": true, ".kt": true, ".rb": true, ".cs": true, ".c": true,
		".cpp": true, ".h": true, ".php": true,
	}
	files := make([]githubTreeEntry, 0)
	for _, entry := range tree.Tree {
		if entry.Type != "blob" || entry.Size <= 0 || entry.Size > 64<<10 {
			continue
		}
		lower := strings.ToLower(entry.Path)
		if strings.Contains(lower, "vendor/") || strings.Contains(lower, "node_modules/") ||
			strings.Contains(lower, "dist/") || strings.Contains(lower, "build/") ||
			strings.HasPrefix(lower, ".") || strings.Contains(lower, "/.") {
			continue
		}
		if !extensions[path.Ext(lower)] {
			continue
		}
		files = append(files, entry)
	}
	sort.Slice(files, func(i, j int) bool {
		left, right := strings.Count(files[i].Path, "/"), strings.Count(files[j].Path, "/")
		if left != right {
			return left < right
		}
		return files[i].Path < files[j].Path
	})
	if len(files) > max {
		files = files[:max]
	}
	return files
}

func escapeRepoSegments(repo string) string {
	parts := strings.Split(repo, "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}

func escapePath(value string) string {
	parts := strings.Split(value, "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
