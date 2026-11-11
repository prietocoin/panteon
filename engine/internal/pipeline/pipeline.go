package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"elpanteon/engine/internal/atlas"
	"elpanteon/engine/internal/cervantes"
	"elpanteon/engine/internal/davinci"
	"elpanteon/engine/internal/encoder"
	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/galileo/store"
	"elpanteon/engine/internal/mozart"
	"elpanteon/engine/internal/pavarotti"
	"elpanteon/engine/internal/pixar"
	"elpanteon/engine/internal/socrates"
)

// Progress es un evento de avance del pipeline para la UI.
type Progress struct {
	Phase      string `json:"phase"`
	Message    string `json:"message"`
	Clip       int    `json:"clip"`
	TotalClips int    `json:"total_clips"`
	Percent    int    `json:"percent"`
}

// Config controla una ejecución completa del pipeline.
type Config struct {
	OutDir      string
	ClipSeconds float64
	FPS         int
	Width       int
	Height      int
	Voice       string
	SynthName   string
	Renderer    pixar.Renderer
	Encoder     encoder.Encoder
	Synth       pavarotti.Synthesizer
	Mixer       mozart.Mixer
	Atlas       atlas.Builder
	ClipCopies  int
	SceneMode   bool
	AllowReview bool
	Progress    func(Progress)
	Now         func() time.Time
}

// Pipeline orquesta las fases desde un engineering_case hasta el contenedor final.
type Pipeline struct {
	config Config
}

// New valida y normaliza la configuración.
func New(config Config) (*Pipeline, error) {
	if config.OutDir == "" {
		config.OutDir = filepath.Join("out", "run")
	}
	if config.ClipSeconds <= 0 {
		config.ClipSeconds = cervantes.DefaultClipSeconds
	}
	if config.FPS <= 0 {
		config.FPS = 12
	}
	if config.Width <= 0 {
		config.Width = 640
	}
	if config.Height <= 0 {
		config.Height = 360
	}
	if config.Renderer == nil {
		config.Renderer = pixar.NewRaster()
	}
	if config.Encoder == nil {
		config.Encoder = encoder.Select("auto")
	}
	if config.Synth == nil {
		config.Synth = pavarotti.ToneSynth{}
	}
	if config.Mixer == nil {
		config.Mixer = mozart.WAVMixer{}
	}
	if config.SynthName == "" {
		config.SynthName = "tone"
	}
	if config.Voice == "" {
		config.Voice = "default"
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.Atlas == nil {
		config.Atlas = atlas.DeterministicBuilder{Now: config.Now}
	}
	if config.ClipCopies <= 0 {
		config.ClipCopies = 1
	}
	if config.FPS > 120 || config.Width > 7680 || config.Height > 4320 {
		return nil, fmt.Errorf("pipeline: configuración fuera de rango")
	}
	return &Pipeline{config: config}, nil
}

// Run ejecuta todas las fases de principio a fin.
func (p *Pipeline) Run(ctx context.Context, engineeringCase contract.EngineeringCase) (contract.RunManifest, error) {
	if err := engineeringCase.Validate(); err != nil {
		return contract.RunManifest{}, fmt.Errorf("pipeline: %w", err)
	}

	out := p.config.OutDir
	for _, dir := range []string{
		out,
		filepath.Join(out, "html"),
		filepath.Join(out, "audio"),
		filepath.Join(out, "clips"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return contract.RunManifest{}, fmt.Errorf("pipeline: creando %s: %w", dir, err)
		}
	}

	started := p.config.Now().UTC()
	report := func(phase, message string, clip, total, percent int) {
		if p.config.Progress != nil {
			p.config.Progress(Progress{
				Phase:      phase,
				Message:    message,
				Clip:       clip,
				TotalClips: total,
				Percent:    percent,
			})
		}
	}

	rigor := socrates.Review(engineeringCase)
	if !rigor.Passed && !p.config.AllowReview {
		return contract.RunManifest{}, fmt.Errorf(
			"pipeline: Sócrates no aprobó el caso (score %d); usa --allow-review para forzar",
			rigor.Score,
		)
	}
	report("rigor", fmt.Sprintf("Sócrates: %d/100", rigor.Score), 0, 0, 3)

	dossier, err := p.config.Atlas.Build(ctx, engineeringCase)
	if err != nil {
		return contract.RunManifest{}, err
	}
	if err := writeJSON(filepath.Join(out, "dossier_tecnico.json"), dossier); err != nil {
		return contract.RunManifest{}, err
	}
	report("atlas", "Dossier técnico listo", 0, 0, 6)

	cut, err := cervantes.Write(dossier, cervantes.Options{
		ClipSeconds: p.config.ClipSeconds,
		Copies:      p.config.ClipCopies,
		SceneMode:   p.config.SceneMode,
		Now:         p.config.Now,
	})
	if err != nil {
		return contract.RunManifest{}, err
	}

	clipList := flattenClips(&cut)
	report("cervantes", fmt.Sprintf("%d clips atómicos", len(clipList)), 0, len(clipList), 10)
	clipAudio := make([][]byte, len(clipList))
	clipAudioPath := make([]string, len(clipList))
	for index, clip := range clipList {
		if err := ctx.Err(); err != nil {
			return contract.RunManifest{}, err
		}
		audio, path, err := p.synthesize(ctx, *clip)
		if err != nil {
			return contract.RunManifest{}, err
		}
		duration := clip.DurationSec
		if probed, err := pavarotti.Duration(audio); err == nil && probed > duration {
			duration = probed
		}
		if duration <= 0 || duration > 600 {
			return contract.RunManifest{}, fmt.Errorf("pipeline: duración inválida para %s: %.2fs", clip.ID, duration)
		}
		clip.DurationSec = duration
		clipAudio[index] = audio
		clipAudioPath[index] = path
		report("audio", fmt.Sprintf("Audio %s (%d/%d)", clip.ID, index+1, len(clipList)), index+1, len(clipList), 10+10*(index+1)/len(clipList))
	}

	if err := writeJSON(filepath.Join(out, "director_cut.json"), cut); err != nil {
		return contract.RunManifest{}, err
	}

	frames := make([]string, 0)
	clipManifests := make([]contract.ClipManifest, 0, len(clipList))
	for index, clip := range clipList {
		if err := ctx.Err(); err != nil {
			return contract.RunManifest{}, err
		}
		page, err := davinci.RenderClip(*clip, filepath.Join(out, "html"), p.config.Width, p.config.Height)
		if err != nil {
			return contract.RunManifest{}, err
		}

		renderKey, err := p.renderKey(*clip, page)
		if err != nil {
			return contract.RunManifest{}, err
		}
		clipFrames, reused, err := p.render(ctx, *clip, page, renderKey)
		if err != nil {
			return contract.RunManifest{}, err
		}
		frames = append(frames, clipFrames...)
		clipManifests = append(clipManifests, contract.ClipManifest{
			ID:          clip.ID,
			Narration:   clip.Narration,
			DurationSec: clip.DurationSec,
			Frames:      len(clipFrames),
			HTML:        page.Path,
			Audio:       clipAudioPath[index],
			CacheKey:    renderKey,
			Reused:      reused,
		})
		report("render", fmt.Sprintf("Render %s (%d/%d)", clip.ID, index+1, len(clipList)), index+1, len(clipList), 20+70*(index+1)/len(clipList))
	}

	audioOut := filepath.Join(out, "audio", "master.wav")
	tracks := make([]mozart.Track, 0, len(clipList))
	offset := 0.0
	for index, clip := range clipList {
		tracks = append(tracks, mozart.Track{Data: clipAudio[index], OffsetSec: offset})
		offset += clip.DurationSec
	}
	report("mix", "Mezclando audio y música", 0, 0, 92)
	if err := p.config.Mixer.Mix(ctx, tracks, offset, audioOut); err != nil {
		return contract.RunManifest{}, err
	}

	report("encode", "Codificando vídeo final", 0, 0, 95)
	masterPath, err := p.config.Encoder.Encode(
		ctx,
		frames,
		audioOut,
		filepath.Join(out, "master"),
		p.config.FPS,
	)
	if err != nil {
		return contract.RunManifest{}, err
	}

	manifest := contract.RunManifest{
		SchemaVersion: contract.ManifestSchemaVersion,
		CaseID:        engineeringCase.ID,
		Title:         engineeringCase.Title,
		StartedAt:     started,
		FinishedAt:    p.config.Now().UTC(),
		FPS:           p.config.FPS,
		Width:         p.config.Width,
		Height:        p.config.Height,
		FrameCount:    len(frames),
		Rigor:         rigor,
		Clips:         clipManifests,
		Outputs: []contract.Output{
			{Kind: "dossier", Path: filepath.Join(out, "dossier_tecnico.json")},
			{Kind: "director_cut", Path: filepath.Join(out, "director_cut.json")},
			{Kind: "audio", Path: audioOut},
			{Kind: "video", Path: masterPath},
		},
	}
	if err := manifest.Validate(); err != nil {
		return contract.RunManifest{}, fmt.Errorf("pipeline: %w", err)
	}
	if err := writeJSON(filepath.Join(out, "run_manifest.json"), manifest); err != nil {
		return contract.RunManifest{}, err
	}
	report("done", "Render completo", 0, 0, 100)
	return manifest, nil
}

func (p *Pipeline) synthesize(ctx context.Context, clip contract.Clip) ([]byte, string, error) {
	key := store.CacheKey("tts", p.config.SynthName, p.config.Voice, clip.Narration, fmt.Sprintf("%.3f", clip.DurationSec))
	path := filepath.Join(p.config.OutDir, "audio", safeName(clip.ID)+".wav")
	keyPath := path + ".key"

	if matchesKey(keyPath, key) {
		if body, err := os.ReadFile(path); err == nil {
			if _, err := pavarotti.DecodeWAV(body); err == nil {
				return body, path, nil
			}
		}
	}

	body, err := p.config.Synth.Synthesize(ctx, pavarotti.Request{
		Text:        clip.Narration,
		Voice:       p.config.Voice,
		DurationSec: clip.DurationSec,
	})
	if err != nil {
		return nil, "", fmt.Errorf("pipeline: sintetizando %s: %w", clip.ID, err)
	}
	if err := writeFileAtomic(path, body); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(keyPath, []byte(key), 0o644); err != nil {
		return nil, "", err
	}
	return body, path, nil
}

func (p *Pipeline) renderKey(clip contract.Clip, page davinci.Page) (string, error) {
	body, err := json.Marshal(clip)
	if err != nil {
		return "", fmt.Errorf("pipeline: serializando clip: %w", err)
	}
	return store.CacheKey(
		"render",
		p.config.Renderer.Name(),
		p.config.Renderer.Version(),
		fmt.Sprintf("%dx%d@%d", p.config.Width, p.config.Height, p.config.FPS),
		string(body),
		contract.ContentHash(page.HTML),
	), nil
}

func (p *Pipeline) render(ctx context.Context, clip contract.Clip, page davinci.Page, key string) ([]string, bool, error) {
	expected := int(math.Ceil(clip.DurationSec * float64(p.config.FPS)))
	if expected < 1 {
		expected = 1
	}
	dir := filepath.Join(p.config.OutDir, "clips", safeName(clip.ID), "frames")
	keyPath := filepath.Join(p.config.OutDir, "clips", safeName(clip.ID), "render.key")

	if matchesKey(keyPath, key) {
		if paths := existingFrames(dir, expected); paths != nil {
			return paths, true, nil
		}
	}

	paths, err := p.config.Renderer.Render(ctx, clip, page, pixar.Options{
		Width:      p.config.Width,
		Height:     p.config.Height,
		FPS:        p.config.FPS,
		OutDir:     filepath.Join(p.config.OutDir, "clips"),
		ChromePath: "",
	})
	if err != nil {
		return nil, false, err
	}
	if len(paths) != expected {
		return nil, false, fmt.Errorf("pipeline: %s produjo %d frames, se esperaban %d", clip.ID, len(paths), expected)
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o755); err != nil {
		return nil, false, err
	}
	if err := os.WriteFile(keyPath, []byte(key), 0o644); err != nil {
		return nil, false, err
	}
	return paths, false, nil
}

func existingFrames(dir string, expected int) []string {
	paths := make([]string, 0, expected)
	for index := 0; index < expected; index++ {
		path := filepath.Join(dir, fmt.Sprintf("frame_%05d.png", index))
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			return nil
		}
		paths = append(paths, path)
	}
	return paths
}

func matchesKey(path, key string) bool {
	body, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(body)) == key
}

func flattenClips(cut *contract.DirectorCut) []*contract.Clip {
	clips := make([]*contract.Clip, 0)
	for sceneIndex := range cut.Scenes {
		scene := &cut.Scenes[sceneIndex]
		for clipIndex := range scene.Clips {
			clips = append(clips, &scene.Clips[clipIndex])
		}
	}
	return clips
}

func writeJSON(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("pipeline: serializando %s: %w", filepath.Base(path), err)
	}
	body = append(body, '\n')
	return writeFileAtomic(path, body)
}

func writeFileAtomic(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("pipeline: creando directorio: %w", err)
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, body, 0o644); err != nil {
		return fmt.Errorf("pipeline: escribiendo %s: %w", temporary, err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("pipeline: publicando %s: %w", path, err)
	}
	return nil
}

func safeName(value string) string {
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, value)
	name = strings.Trim(name, "-")
	if name == "" {
		return "clip"
	}
	return name
}
