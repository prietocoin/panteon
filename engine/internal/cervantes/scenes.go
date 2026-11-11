package cervantes

import (
	"fmt"
	"strings"
	"time"

	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/textutil"
)

// writeSceneCut genera escenas/bloques de duración variable según la narración.
// La duración final la confirma Pavarotti al sintetizar cada escena.
func writeSceneCut(dossier contract.Dossier, opts Options) (contract.DirectorCut, error) {
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
	mechanism := artifactContent(dossier, "cause-mechanism")
	solutionCode := artifactContent(dossier, "solution-code")
	repoCode := artifactContent(dossier, "repo-code")
	actions := actionLines(solutionCode)

	if strings.TrimSpace(mechanism) == "" {
		mechanism = cause
	}
	if strings.TrimSpace(evidence) == "" {
		evidence = symptom
	}
	code := solutionCode
	if strings.TrimSpace(repoCode) != "" {
		code = repoCode
	}
	if strings.TrimSpace(code) == "" {
		code = "# Sin código disponible"
	}

	cut := contract.DirectorCut{
		SchemaVersion: contract.CutSchemaVersion,
		CaseID:        dossier.CaseID,
		Title:         dossier.Title,
		GeneratedAt:   now().UTC(),
	}

	addScene := func(title, narration string, beats []contract.Beat) {
		clipID := fmt.Sprintf("clip-%02d", len(cut.Scenes)+1)
		cut.Scenes = append(cut.Scenes, contract.Scene{
			ID:    fmt.Sprintf("scene-%02d", len(cut.Scenes)+1),
			Title: title,
			Clips: []contract.Clip{{
				ID:          clipID,
				Narration:   textutil.Clean(narration),
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

	addScene(
		"El caso",
		joinNarration(
			dossier.Title+".",
			symptom,
			"Este documental reconstruye el incidente a partir de evidencia primaria: qué falló, por qué falló y cómo se corrigió.",
			"La fuente es "+sourceLabel(dossier)+".",
		),
		[]contract.Beat{
			{Start: 0, End: 0.18, Kind: "title", Title: dossier.Title, Subtitle: "El Panteón · Caso de ingeniería"},
			{Start: 0.18, End: 0.58, Kind: "cards", Title: "El caso en 30 segundos", Cards: []contract.Card{
				{Title: "Síntoma", Body: textutil.Truncate(textutil.Clean(symptom), 170), Tone: "danger"},
				{Title: "Fuente", Body: textutil.Truncate(textutil.Clean(sourceLabel(dossier)), 170), Tone: "neutral"},
			}},
			{Start: 0.58, End: 1, Kind: "bullets", Title: "Qué vamos a reconstruir", Bullets: []string{
				"Qué se observó y cuánto dolió.",
				"Qué mecanismo lo provocó.",
				"Qué se cambió y cómo se verificó.",
			}},
		},
	)

	addScene(
		"El síntoma",
		joinNarration(
			"El síntoma.",
			symptom,
			"La evidencia literal de la fuente dice: "+evidence+".",
			"Un incidente no empieza por la causa: empieza por una señal observable.",
		),
		[]contract.Beat{
			{Start: 0, End: 0.12, Kind: "title", Title: "El síntoma", Subtitle: "La señal observable"},
			{Start: 0.12, End: 0.5, Kind: "cards", Title: "Lo que se observó", Cards: []contract.Card{
				{Title: "Síntoma", Body: textutil.Truncate(textutil.Clean(symptom), 180), Tone: "danger"},
				{Title: "Evidencia", Body: textutil.Truncate(textutil.Clean(evidence), 180), Tone: "warning"},
			}},
			{Start: 0.5, End: 0.78, Kind: "bullets", Title: "Señales y métricas", Bullets: textutil.Bullets(symptom+" "+evidence, 4)},
			{Start: 0.78, End: 1, Kind: "bullets", Title: "Por qué importa", Bullets: []string{
				"No es una opinión: es una medición.",
				"El síntoma define el alcance del caso.",
			}},
		},
	)

	addScene(
		"La evidencia",
		joinNarration(
			"La evidencia.",
			evidence,
			"Esto es lo que la fuente permite afirmar sin inventar.",
			"El caso se sostiene sobre citas y localizadores, no sobre interpretaciones.",
		),
		[]contract.Beat{
			{Start: 0, End: 0.12, Kind: "title", Title: "La evidencia", Subtitle: "Citas y procedencia"},
			{Start: 0.12, End: 0.62, Kind: "bullets", Title: "Lo que dice la fuente", Bullets: textutil.Bullets(evidence, 5)},
			{Start: 0.62, End: 1, Kind: "cards", Title: "Procedencia", Cards: []contract.Card{
				{Title: "Fuente", Body: textutil.Truncate(textutil.Clean(sourceLabel(dossier)), 180), Tone: "neutral"},
				{Title: "Criterio", Body: "Cada afirmación debe poder rastrearse hasta una cita literal.", Tone: "success"},
			}},
		},
	)

	causeBullets := textutil.Bullets(cause+" "+mechanism, 5)
	if len(causeBullets) == 0 {
		causeBullets = []string{cause}
	}
	addScene(
		"La causa",
		joinNarration(
			"La causa raíz.",
			cause,
			"El mecanismo técnico es el siguiente: "+mechanism+".",
			"Una causa no es una etiqueta: es una cadena de eventos que explica el síntoma.",
		),
		[]contract.Beat{
			{Start: 0, End: 0.12, Kind: "title", Title: "La causa", Subtitle: "Del síntoma al mecanismo"},
			{Start: 0.12, End: 0.55, Kind: "bullets", Title: "Qué falló", Bullets: causeBullets},
			{Start: 0.55, End: 0.78, Kind: "cards", Title: "Lectura técnica", Cards: []contract.Card{
				{Title: "Mecanismo", Body: textutil.Truncate(textutil.Clean(mechanism), 200), Tone: "warning"},
				{Title: "Condición", Body: "El fallo aparece cuando coinciden carga, estado y una decisión de diseño.", Tone: "neutral"},
			}},
			{Start: 0.78, End: 1, Kind: "code", Title: "Código o configuración relevante", Code: textutil.Truncate(code, 1400), CodeLanguage: "source"},
		},
	)

	solutionCards := make([]contract.Card, 0, 4)
	for index, action := range actions {
		if index == 4 {
			break
		}
		solutionCards = append(solutionCards, contract.Card{
			Title: fmt.Sprintf("Acción %d", index+1),
			Body:  textutil.Truncate(textutil.Clean(action), 150),
			Tone:  "success",
		})
	}
	if len(solutionCards) == 0 {
		solutionCards = append(solutionCards, contract.Card{
			Title: "Remediación",
			Body:  textutil.Truncate(textutil.Clean(solution), 150),
			Tone:  "success",
		})
	}
	addScene(
		"La solución",
		joinNarration(
			"La solución.",
			solution,
			"Las acciones aplicadas fueron: "+strings.Join(actions, "; ")+".",
			"Los trade-offs declarados son: "+tradeoffs+".",
			"Una solución sin compensaciones no es una solución completa.",
		),
		[]contract.Beat{
			{Start: 0, End: 0.12, Kind: "title", Title: "La solución", Subtitle: "Qué se cambió"},
			{Start: 0.12, End: 0.48, Kind: "cards", Title: "Plan de remediación", Cards: solutionCards},
			{Start: 0.48, End: 0.68, Kind: "bullets", Title: "Trade-offs", Bullets: textutil.Bullets(tradeoffs, 3)},
			{Start: 0.68, End: 1, Kind: "code", Title: "Cambios aplicados", Code: textutil.Truncate(solutionCode, 1600), CodeLanguage: "yaml"},
		},
	)

	addScene(
		"Verificación y cierre",
		joinNarration(
			"La verificación.",
			verification,
			"El caso se cierra con evidencia de que la corrección funcionó.",
			"Sin hype, con evidencia. Fin del caso.",
		),
		[]contract.Beat{
			{Start: 0, End: 0.12, Kind: "title", Title: "La verificación", Subtitle: "Cómo sabemos que funcionó"},
			{Start: 0.12, End: 0.55, Kind: "bullets", Title: "Resultado", Bullets: textutil.Bullets(verification, 4)},
			{Start: 0.55, End: 0.8, Kind: "bullets", Title: "Compensaciones", Bullets: textutil.Bullets(tradeoffs, 3)},
			{Start: 0.8, End: 1, Kind: "title", Title: "Fin del caso", Subtitle: "El Panteón · Sin Hype"},
		},
	)

	if err := cut.Validate(); err != nil {
		return contract.DirectorCut{}, fmt.Errorf("cervantes: %w", err)
	}
	return cut, nil
}

func joinNarration(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = textutil.Clean(part)
		if part == "" {
			continue
		}
		cleaned = append(cleaned, part)
	}
	return strings.Join(cleaned, " ")
}

func estimateNarrationSeconds(text string) float64 {
	words := len(strings.Fields(text))
	if words == 0 {
		return 8
	}
	seconds := float64(words)/2.7 + 1
	if seconds < 8 {
		seconds = 8
	}
	return seconds
}

func sourceLabel(dossier contract.Dossier) string {
	if strings.TrimSpace(dossier.CaseID) != "" {
		return "el caso " + dossier.CaseID
	}
	return "la fuente primaria"
}
