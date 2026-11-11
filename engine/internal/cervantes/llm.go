package cervantes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/textutil"
)

// ScriptWriter es un guionista alternativo (LLM) para Cervantes.
type ScriptWriter interface {
	Write(ctx context.Context, dossier contract.Dossier, opts Options) (contract.DirectorCut, error)
}

// LLMWriter llama a un endpoint OpenAI-compatible: DeepSeek, Kimi, Gemini, Ollama, etc.
type LLMWriter struct {
	BaseURL string
	Model   string
	APIKey  string
	Client  *http.Client
}

// NewLLMWriterFromEnv devuelve nil si faltan variables.
func NewLLMWriterFromEnv() *LLMWriter {
	base := strings.TrimSpace(os.Getenv("PANTEON_LLM_BASE_URL"))
	model := strings.TrimSpace(os.Getenv("PANTEON_LLM_MODEL"))
	key := strings.TrimSpace(os.Getenv("PANTEON_LLM_API_KEY"))
	if base == "" || model == "" || key == "" {
		return nil
	}
	return &LLMWriter{BaseURL: base, Model: model, APIKey: key}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type llmScript struct {
	Scenes []struct {
		Title     string `json:"title"`
		Narration string `json:"narration"`
		Beats     []struct {
			Start        float64         `json:"start"`
			End          float64         `json:"end"`
			Kind         string          `json:"kind"`
			Title        string          `json:"title"`
			Subtitle     string          `json:"subtitle"`
			Bullets      []string        `json:"bullets"`
			Cards        []contract.Card `json:"cards"`
			Code         string          `json:"code"`
			CodeLanguage string          `json:"code_language"`
		} `json:"beats"`
	} `json:"scenes"`
}

// Write implementa ScriptWriter.
func (w *LLMWriter) Write(ctx context.Context, dossier contract.Dossier, opts Options) (contract.DirectorCut, error) {
	if strings.TrimSpace(w.APIKey) == "" {
		return contract.DirectorCut{}, errors.New("llm: falta API key")
	}
	target := opts.TargetMinutes
	if target <= 0 {
		target = 12
	}
	dossierJSON, err := json.Marshal(dossier)
	if err != nil {
		return contract.DirectorCut{}, err
	}
	if len(dossierJSON) > 16000 {
		dossierJSON = dossierJSON[:16000]
	}

	system := `Eres Cervantes, guionista técnico de El Panteón. Escribes documentales de arquitectura de software en español, sin hype y con evidencia.
Devuelve SOLO un objeto JSON válido, sin markdown ni explicaciones.
Formato:
{"scenes":[{"title":"...","narration":"...","beats":[{"start":0.0,"end":0.2,"kind":"title","title":"...","subtitle":"..."},{"start":0.2,"end":0.6,"kind":"bullets","title":"...","bullets":["..."]},{"start":0.6,"end":1.0,"kind":"code","title":"...","code":"...","code_language":"yaml"}]}]}
Reglas:
- 6 a 8 escenas; cada escena es un bloque narrativo de 60 a 180 segundos.
- La duración total objetivo la da el usuario. Para 12 minutos escribe ~1800-2200 palabras de narración.
- La narración va en español natural, sin repetir frases ni rellenar con obviedades.
- Cada beat usa kind title, cards, bullets o code. start/end son fracciones 0..1 de la escena.
- Usa SOLO hechos del dossier. No inventes métricas, causas ni citas.
- Cada afirmación técnica debe apoyarse en el dossier. Si falta información, dilo.
- No uses lenguaje promocional: revolucionario, 10x, game changer, sin esfuerzo, etc.`

	user := fmt.Sprintf("Duración objetivo: %d minutos.\nDossier técnico (JSON):\n%s", target, string(dossierJSON))
	payload, err := json.Marshal(chatRequest{
		Model:       w.Model,
		Temperature: 0.4,
		MaxTokens:   8000,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return contract.DirectorCut{}, err
	}

	endpoint := strings.TrimRight(w.BaseURL, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return contract.DirectorCut{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+w.APIKey)

	client := w.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	response, err := client.Do(request)
	if err != nil {
		return contract.DirectorCut{}, fmt.Errorf("llm: %w", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return contract.DirectorCut{}, fmt.Errorf("llm: %s: %s", response.Status, textutil.Truncate(string(body), 300))
	}

	var completion chatResponse
	if err := json.Unmarshal(body, &completion); err != nil {
		return contract.DirectorCut{}, fmt.Errorf("llm: respuesta inválida: %w", err)
	}
	if len(completion.Choices) == 0 {
		return contract.DirectorCut{}, errors.New("llm: respuesta sin choices")
	}
	content := cleanJSON(completion.Choices[0].Message.Content)
	var script llmScript
	if err := json.Unmarshal([]byte(content), &script); err != nil {
		return contract.DirectorCut{}, fmt.Errorf("llm: JSON del guion inválido: %w", err)
	}
	if len(script.Scenes) < 2 {
		return contract.DirectorCut{}, errors.New("llm: guion con muy pocas escenas")
	}

	cut := contract.DirectorCut{
		SchemaVersion: contract.CutSchemaVersion,
		CaseID:        dossier.CaseID,
		Title:         dossier.Title,
		GeneratedAt:   time.Now().UTC(),
	}
	for index, scene := range script.Scenes {
		title := strings.TrimSpace(scene.Title)
		if title == "" {
			title = fmt.Sprintf("Escena %d", index+1)
		}
		narration := strings.TrimSpace(scene.Narration)
		if narration == "" {
			continue
		}
		beats := make([]contract.Beat, 0, len(scene.Beats))
		for _, beat := range scene.Beats {
			start, end := beat.Start, beat.End
			if start > 1 || end > 1 {
				start /= 100
				end /= 100
			}
			if start < 0 {
				start = 0
			}
			if end > 1 {
				end = 1
			}
			if start >= end {
				start, end = 0, 1
			}
			kind := strings.ToLower(strings.TrimSpace(beat.Kind))
			switch kind {
			case "title", "cards", "bullets", "code":
			default:
				kind = "title"
			}
			beatTitle := strings.TrimSpace(beat.Title)
			if beatTitle == "" {
				beatTitle = title
			}
			for cardIndex := range beat.Cards {
				if strings.TrimSpace(beat.Cards[cardIndex].Tone) == "" {
					beat.Cards[cardIndex].Tone = "neutral"
				}
			}
			beats = append(beats, contract.Beat{
				Start:        start,
				End:          end,
				Kind:         kind,
				Title:        beatTitle,
				Subtitle:     beat.Subtitle,
				Bullets:      beat.Bullets,
				Cards:        beat.Cards,
				Code:         beat.Code,
				CodeLanguage: beat.CodeLanguage,
			})
		}
		if len(beats) == 0 {
			beats = []contract.Beat{{Start: 0, End: 1, Kind: "title", Title: title}}
		}
		cut.Scenes = append(cut.Scenes, contract.Scene{
			ID:    fmt.Sprintf("scene-%02d", len(cut.Scenes)+1),
			Title: title,
			Clips: []contract.Clip{{
				ID:          fmt.Sprintf("clip-%02d", len(cut.Scenes)+1),
				Narration:   narration,
				DurationSec: estimateNarrationSeconds(narration),
				Visual: contract.Visual{
					Kind:  "beats",
					Title: title,
					Beats: beats,
				},
				SFX: []string{"whoosh"},
			}},
		})
	}
	if len(cut.Scenes) < 2 {
		return contract.DirectorCut{}, errors.New("llm: guion sin escenas válidas")
	}
	if err := cut.Validate(); err != nil {
		return contract.DirectorCut{}, fmt.Errorf("llm: %w", err)
	}
	return cut, nil
}

func cleanJSON(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		return content[start : end+1]
	}
	return content
}
