package cervantes

import (
	"fmt"
	"strings"
	"time"

	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/textutil"
)

// DefaultClipSeconds es la duración base de un clip atómico en el demo offline.
const DefaultClipSeconds = 2.5

// Options controla el guion.
type Options struct {
	ClipSeconds float64
	// Copies repite la secuencia completa para pruebas de carga o vídeos largos.
	Copies int
	// SceneMode genera escenas/bloques de duración variable según la narración.
	SceneMode bool
	Now       func() time.Time
}

// Write convierte un dossier técnico en un director_cut con escenas y clips atómicos.
func Write(dossier contract.Dossier, opts Options) (contract.DirectorCut, error) {
	if err := dossier.Validate(); err != nil {
		return contract.DirectorCut{}, fmt.Errorf("cervantes: %w", err)
	}
	if opts.SceneMode {
		return writeSceneCut(dossier, opts)
	}

	clipSeconds := opts.ClipSeconds
	if clipSeconds <= 0 {
		clipSeconds = DefaultClipSeconds
	}
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}

	symptom := factValue(dossier, "Síntoma")
	cause := factValue(dossier, "Causa")
	solution := factValue(dossier, "Solución")
	verification := factValue(dossier, "Verificación")
	tradeoffs := factValue(dossier, "Trade-offs")
	evidence := artifactContent(dossier, "symptom-evidence")
	actions := actionLines(artifactContent(dossier, "solution-code"))
	code := artifactContent(dossier, "solution-code")
	if strings.TrimSpace(code) == "" {
		code = "# Sin código disponible"
	}
	repoCode := artifactContent(dossier, "repo-code")

	cut := contract.DirectorCut{
		SchemaVersion: contract.CutSchemaVersion,
		CaseID:        dossier.CaseID,
		Title:         dossier.Title,
		GeneratedAt:   now().UTC(),
	}

	clipID := 0
	addClip := func(scene *contract.Scene, clip contract.Clip) {
		clipID++
		clip.ID = fmt.Sprintf("clip-%02d", clipID)
		clip.DurationSec = clipSeconds
		if strings.TrimSpace(clip.Narration) == "" {
			clip.Narration = clip.Visual.Title
		}
		scene.Clips = append(scene.Clips, clip)
	}

	titleScene := contract.Scene{ID: "scene-01", Title: "El caso"}
	addClip(&titleScene, contract.Clip{
		Narration: "Caso de ingeniería: " + dossier.Title + ".",
		Visual: contract.Visual{
			Kind:     "title",
			Title:    dossier.Title,
			Subtitle: "El Panteón · Caso de ingeniería",
		},
		SFX: []string{"whoosh"},
	})
	addClip(&titleScene, contract.Clip{
		Narration: textutil.FirstSentence(symptom),
		Visual: contract.Visual{
			Kind:  "cards",
			Title: "El síntoma",
			Cards: []contract.Card{
				{Title: "Síntoma", Body: textutil.Truncate(textutil.Clean(symptom), 150), Tone: "danger"},
				{Title: "Evidencia", Body: textutil.Truncate(textutil.Clean(evidence), 150), Tone: "warning"},
			},
		},
		SFX: []string{"impact"},
	})

	causeScene := contract.Scene{ID: "scene-02", Title: "La causa"}
	addClip(&causeScene, contract.Clip{
		Narration: "La causa raíz.",
		Visual: contract.Visual{
			Kind:     "title",
			Title:    "La causa raíz",
			Subtitle: "Del síntoma al mecanismo",
		},
		SFX: []string{"whoosh"},
	})
	causeBullets := textutil.Bullets(cause, 4)
	if len(causeBullets) == 0 {
		causeBullets = []string{cause}
	}
	addClip(&causeScene, contract.Clip{
		Narration: textutil.FirstSentence(cause),
		Visual: contract.Visual{
			Kind:    "bullets",
			Title:   "Qué falló",
			Bullets: causeBullets,
		},
		SFX: []string{"impact"},
	})

	solutionScene := contract.Scene{ID: "scene-03", Title: "La solución"}
	addClip(&solutionScene, contract.Clip{
		Narration: "La solución aplicada.",
		Visual: contract.Visual{
			Kind:     "title",
			Title:    "La solución",
			Subtitle: textutil.Truncate(textutil.Clean(solution), 90),
		},
		SFX: []string{"whoosh"},
	})
	cards := make([]contract.Card, 0, 4)
	for index, action := range actions {
		if index == 4 {
			break
		}
		cards = append(cards, contract.Card{
			Title: fmt.Sprintf("Acción %d", index+1),
			Body:  textutil.Truncate(textutil.Clean(action), 130),
			Tone:  "success",
		})
	}
	if len(cards) == 0 {
		cards = append(cards, contract.Card{
			Title: "Remediación",
			Body:  textutil.Truncate(textutil.Clean(solution), 130),
			Tone:  "success",
		})
	}
	addClip(&solutionScene, contract.Clip{
		Narration: actionNarration(actions, solution),
		Visual: contract.Visual{
			Kind:  "cards",
			Title: "Plan de remediación",
			Cards: cards,
		},
		SFX: []string{"impact"},
	})
	if strings.TrimSpace(repoCode) != "" {
		addClip(&solutionScene, contract.Clip{
			Narration: "Una muestra del código real analizado en el repositorio.",
			Visual: contract.Visual{
				Kind:         "code",
				Title:        "Código del repositorio",
				Code:         textutil.Truncate(repoCode, 1800),
				CodeLanguage: "source",
			},
			SFX: []string{"typing"},
		})
	}
	addClip(&solutionScene, contract.Clip{
		Narration: "Los cambios concretos.",
		Visual: contract.Visual{
			Kind:         "code",
			Title:        "Cambios aplicados",
			Code:         code,
			CodeLanguage: "yaml",
		},
		SFX: []string{"typing"},
	})

	verificationScene := contract.Scene{ID: "scene-04", Title: "La verificación"}
	verificationBullets := textutil.Bullets(verification, 3)
	if tradeoffs != "" {
		verificationBullets = append(verificationBullets, "Trade-off: "+textutil.Truncate(textutil.Clean(tradeoffs), 140))
	}
	if len(verificationBullets) == 0 {
		verificationBullets = []string{"Verificación no declarada en la fuente."}
	}
	addClip(&verificationScene, contract.Clip{
		Narration: textutil.FirstSentence(verification),
		Visual: contract.Visual{
			Kind:    "bullets",
			Title:   "Cómo se verificó",
			Bullets: verificationBullets,
		},
		SFX: []string{"impact"},
	})
	addClip(&verificationScene, contract.Clip{
		Narration: "Fin del caso. Sin hype, con evidencia.",
		Visual: contract.Visual{
			Kind:     "title",
			Title:    "Fin del caso",
			Subtitle: "El Panteón · Sin Hype",
		},
		SFX: []string{"whoosh"},
	})

	cut.Scenes = []contract.Scene{titleScene, causeScene, solutionScene, verificationScene}
	if opts.Copies > 1 {
		cut.Scenes = expandScenes(cut.Scenes, opts.Copies)
	}
	if err := cut.Validate(); err != nil {
		return contract.DirectorCut{}, fmt.Errorf("cervantes: %w", err)
	}
	return cut, nil
}

// expandScenes repite la secuencia con IDs únicos para pruebas de carga.
func expandScenes(scenes []contract.Scene, copies int) []contract.Scene {
	out := make([]contract.Scene, 0, len(scenes)*copies)
	clipIndex := 0
	for copyIndex := 0; copyIndex < copies; copyIndex++ {
		for sceneIndex, scene := range scenes {
			expanded := scene
			expanded.ID = fmt.Sprintf("scene-%02d-%02d", copyIndex+1, sceneIndex+1)
			expanded.Clips = make([]contract.Clip, len(scene.Clips))
			for clipOffset, clip := range scene.Clips {
				clipIndex++
				expandedClip := clip
				expandedClip.ID = fmt.Sprintf("clip-%03d", clipIndex)
				expanded.Clips[clipOffset] = expandedClip
			}
			out = append(out, expanded)
		}
	}
	return out
}

func factValue(dossier contract.Dossier, label string) string {
	for _, fact := range dossier.Facts {
		if fact.Label == label {
			return fact.Value
		}
	}
	return ""
}

func artifactContent(dossier contract.Dossier, id string) string {
	for _, artifact := range dossier.Artifacts {
		if artifact.ID == id {
			return artifact.Content
		}
	}
	return ""
}

func actionLines(code string) []string {
	lines := strings.Split(code, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		out = append(out, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
	}
	return out
}

func actionNarration(actions []string, fallback string) string {
	if len(actions) == 0 {
		return textutil.FirstSentence(fallback)
	}
	return "Se aplicaron " + fmt.Sprintf("%d", len(actions)) + " acciones: " + strings.Join(actions, "; ") + "."
}
