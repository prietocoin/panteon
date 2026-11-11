package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"elpanteon/engine/internal/atlas"
	"elpanteon/engine/internal/encoder"
	"elpanteon/engine/internal/galileo"
	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/galileo/extract"
	"elpanteon/engine/internal/galileo/source"
	"elpanteon/engine/internal/keepawake"
	"elpanteon/engine/internal/pavarotti"
	"elpanteon/engine/internal/pipeline"
	"elpanteon/engine/internal/pixar"
	"elpanteon/engine/internal/webui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("falta el comando")
	}

	switch args[0] {
	case "galileo":
		return runGalileo(args[1:])
	case "produce":
		return runProduce(args[1:])
	case "demo":
		return runDemo(args[1:])
	case "serve":
		return runServe(args[1:])
	case "version":
		fmt.Println("panteon 0.2.0")
		return nil
	default:
		usage()
		return fmt.Errorf("comando desconocido: %q", args[0])
	}
}

func runGalileo(args []string) error {
	if len(args) == 0 || args[0] != "run" {
		usage()
		return errors.New("uso: panteon galileo run --file <ruta> | --url <url>")
	}

	flags := flag.NewFlagSet("galileo run", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	file := flags.String("file", "", "fuente local (.md, .txt, .html)")
	rawURL := flags.String("url", "", "URL de la fuente primaria")
	outputDir := flags.String("out", filepath.Join("out", "cases"), "directorio de artefactos")
	sourceType := flags.String("type", "", "tipo de fuente: postmortem, rfc, adr, other")
	title := flags.String("title", "", "título del caso")
	asJSON := flags.Bool("json", false, "imprime el engineering_case.json en stdout")

	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if (*file == "") == (*rawURL == "") {
		return errors.New("indica exactamente una fuente: --file o --url")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var (
		doc source.Document
		err error
	)
	if *file != "" {
		doc, err = source.FromFile(*file, source.Options{Type: *sourceType, Title: *title})
	} else {
		doc, err = source.FromURL(ctx, *rawURL, nil)
	}
	if err != nil {
		return err
	}

	service := galileo.New(extract.MarkedExtractor{}, *outputDir)
	result, err := service.Run(ctx, doc)
	if err != nil {
		return err
	}

	if *asJSON {
		body, err := json.MarshalIndent(result.Case, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	}

	fmt.Printf("Galileo: %s\n", result.Case.Title)
	fmt.Printf("Veredicto: %s (score %d)\n", result.Case.HypeFilter.Verdict, result.Case.HypeFilter.Score)
	fmt.Printf("Caso: %s\n", result.Path)
	fmt.Printf("Cache: %s\n", result.CacheKey)
	if len(result.Case.HypeFilter.Flags) > 0 {
		fmt.Println("Flags:")
		for _, item := range result.Case.HypeFilter.Flags {
			fmt.Printf("  - [%s] %s (-%d)\n", item.Rule, item.Message, item.Penalty)
		}
	}
	if result.Case.HypeFilter.Verdict == contract.VerdictReject {
		fmt.Fprintln(os.Stderr, "aviso: el caso fue rechazado por el filtro Anti-Hype")
	}
	return nil
}

type pipelineFlags struct {
	out         string
	clipSeconds float64
	clipCopies  int
	fps         int
	width       int
	height      int
	renderer    string
	encoder     string
	tts         string
	ttsURL      string
	voice       string
	chrome      string
	atlasMode   string
	repo        string
	ref         string
	githubToken string
	allowReview bool
	timeout     time.Duration
	keepAwake   bool
}

func (f *pipelineFlags) register(flags *flag.FlagSet, defaultOut string) {
	flags.StringVar(&f.out, "out", defaultOut, "directorio de salida")
	flags.Float64Var(&f.clipSeconds, "clip-seconds", 2.5, "duración base de cada clip")
	flags.IntVar(&f.clipCopies, "clip-copies", 1, "número de copias de la secuencia (pruebas de carga)")
	flags.IntVar(&f.fps, "fps", 12, "frames por segundo")
	flags.IntVar(&f.width, "width", 640, "ancho del canvas")
	flags.IntVar(&f.height, "height", 360, "alto del canvas")
	flags.StringVar(&f.renderer, "renderer", "raster", "renderer: raster | chromium")
	flags.StringVar(&f.encoder, "encoder", "auto", "encoder: auto | gif | ffmpeg")
	flags.StringVar(&f.tts, "tts", "tone", "TTS: tone | http")
	flags.StringVar(&f.ttsURL, "tts-url", "", "URL base del servicio TTS para --tts http")
	flags.StringVar(&f.voice, "voice", "default", "voz del TTS")
	flags.StringVar(&f.chrome, "chrome", "", "ruta a chrome.exe para --renderer chromium")
	flags.StringVar(&f.atlasMode, "atlas", "deterministic", "Atlas: deterministic | github")
	flags.StringVar(&f.repo, "repo", "", "repositorio owner/name para --atlas github")
	flags.StringVar(&f.ref, "ref", "", "rama, tag o commit para --atlas github")
	flags.StringVar(&f.githubToken, "github-token", "", "token de GitHub (o variable GITHUB_TOKEN)")
	flags.BoolVar(&f.allowReview, "allow-review", false, "permite producir casos marcados para revisión")
	flags.DurationVar(&f.timeout, "timeout", 3*time.Hour, "timeout total del pipeline")
	flags.BoolVar(&f.keepAwake, "keep-awake", true, "evita la suspensión del sistema durante el render")
}

func (f pipelineFlags) config() (pipeline.Config, error) {
	config := pipeline.Config{
		OutDir:      f.out,
		ClipSeconds: f.clipSeconds,
		ClipCopies:  f.clipCopies,
		FPS:         f.fps,
		Width:       f.width,
		Height:      f.height,
		Voice:       f.voice,
		SynthName:   f.tts,
		AllowReview: f.allowReview,
	}
	switch f.atlasMode {
	case "deterministic":
		config.Atlas = atlas.DeterministicBuilder{}
	case "github":
		token := f.githubToken
		if token == "" {
			token = os.Getenv("GITHUB_TOKEN")
		}
		config.Atlas = atlas.GitHubBuilder{Repo: f.repo, Ref: f.ref, Token: token}
	default:
		return pipeline.Config{}, fmt.Errorf("atlas desconocido: %q", f.atlasMode)
	}
	switch f.renderer {
	case "raster":
		config.Renderer = pixar.NewRaster()
	case "chromium":
		config.Renderer = pixar.NewChromium(f.chrome)
	default:
		return pipeline.Config{}, fmt.Errorf("renderer desconocido: %q", f.renderer)
	}
	switch f.tts {
	case "tone":
		config.Synth = pavarotti.ToneSynth{}
	case "http":
		if f.ttsURL == "" {
			return pipeline.Config{}, errors.New("--tts http requiere --tts-url")
		}
		config.Synth = pavarotti.HTTPSynth{BaseURL: f.ttsURL}
	default:
		return pipeline.Config{}, fmt.Errorf("tts desconocido: %q", f.tts)
	}
	config.Encoder = encoder.Select(f.encoder)
	return config, nil
}

func runProduce(args []string) error {
	flags := flag.NewFlagSet("produce", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	casePath := flags.String("case", "", "ruta al engineering_case.json")
	asJSON := flags.Bool("json", false, "imprime el run_manifest.json en stdout")
	var options pipelineFlags
	options.register(flags, filepath.Join("out", "run"))

	if err := flags.Parse(args); err != nil {
		return err
	}
	if *casePath == "" {
		return errors.New("indica --case <engineering_case.json>")
	}

	body, err := os.ReadFile(*casePath)
	if err != nil {
		return err
	}
	var engineeringCase contract.EngineeringCase
	if err := json.Unmarshal(body, &engineeringCase); err != nil {
		return fmt.Errorf("leyendo caso: %w", err)
	}
	if err := engineeringCase.Validate(); err != nil {
		return err
	}

	config, err := options.config()
	if err != nil {
		return err
	}
	service, err := pipeline.New(config)
	if err != nil {
		return err
	}

	if options.keepAwake {
		keepawake.Enable()
		defer keepawake.Disable()
	}
	ctx, cancel := context.WithTimeout(context.Background(), options.timeout)
	defer cancel()

	manifest, err := service.Run(ctx, engineeringCase)
	if err != nil {
		return err
	}
	if *asJSON {
		encoded, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(encoded))
		return nil
	}
	printManifest(manifest)
	return nil
}

func runDemo(args []string) error {
	flags := flag.NewFlagSet("demo", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	sourcePath := flags.String("source", filepath.Join("engine", "internal", "galileo", "testdata", "postmortem.md"), "fuente del caso demo")
	var options pipelineFlags
	options.register(flags, filepath.Join("out", "demo"))

	if err := flags.Parse(args); err != nil {
		return err
	}

	if options.keepAwake {
		keepawake.Enable()
		defer keepawake.Disable()
	}
	ctx, cancel := context.WithTimeout(context.Background(), options.timeout)
	defer cancel()

	doc, err := source.FromFile(*sourcePath, source.Options{})
	if err != nil {
		return err
	}
	galileoService := galileo.New(extract.MarkedExtractor{}, filepath.Join(options.out, "cases"))
	caseResult, err := galileoService.Run(ctx, doc)
	if err != nil {
		return err
	}
	fmt.Printf("Galileo: %s\n", caseResult.Case.Title)
	fmt.Printf("Veredicto: %s (score %d)\n", caseResult.Case.HypeFilter.Verdict, caseResult.Case.HypeFilter.Score)
	fmt.Printf("Caso: %s\n\n", caseResult.Path)

	options.out = filepath.Join(options.out, "run")
	config, err := options.config()
	if err != nil {
		return err
	}
	service, err := pipeline.New(config)
	if err != nil {
		return err
	}
	manifest, err := service.Run(ctx, caseResult.Case)
	if err != nil {
		return err
	}
	printManifest(manifest)
	return nil
}

func runServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	addr := flags.String("addr", "127.0.0.1:8080", "dirección HTTP (por defecto solo local)")
	workspace := flags.String("workspace", filepath.Join("out", "web"), "directorio de casos, reviews y runs")
	if err := flags.Parse(args); err != nil {
		return err
	}

	server, err := webui.New(*workspace)
	if err != nil {
		return fmt.Errorf("webui: %w", err)
	}
	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		fmt.Printf("El Panteón UI: http://%s\n", displayAddr(*addr))
		fmt.Printf("Workspace: %s\n", *workspace)
		serverErrors <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}
	return nil
}

func displayAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	if strings.HasPrefix(addr, "0.0.0.0:") {
		return "127.0.0.1:" + strings.TrimPrefix(addr, "0.0.0.0:")
	}
	return addr
}

func printManifest(manifest contract.RunManifest) {
	fmt.Printf("Pipeline: %s\n", manifest.Title)
	fmt.Printf("Rigor: %d/100 (aprobado=%v)\n", manifest.Rigor.Score, manifest.Rigor.Passed)
	fmt.Printf("Clips: %d · Frames: %d · %dx%d @ %dfps\n",
		len(manifest.Clips), manifest.FrameCount, manifest.Width, manifest.Height, manifest.FPS)
	fmt.Println("Salidas:")
	for _, output := range manifest.Outputs {
		fmt.Printf("  [%s] %s\n", output.Kind, output.Path)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `El Panteón

Uso:
  panteon galileo run --file <ruta> [--out <dir>] [--type <tipo>] [--title <título>]
  panteon galileo run --url <url>   [--out <dir>] [--json]
  panteon produce --case <engineering_case.json> [--out <dir>] [--renderer raster|chromium]
  panteon produce --case <caso.json> --atlas github --repo owner/name
  panteon demo [--out <dir>] [--renderer raster|chromium] [--encoder auto|gif|ffmpeg]
  panteon produce --case <caso.json> --clip-seconds 20 --clip-copies 4   # 12 min de carga
  panteon serve [--addr 127.0.0.1:8080] [--workspace out/web]
  panteon version

Salida final:
  - Con ffmpeg instalado: master.mp4
  - Sin ffmpeg: master.gif + master.wav`)
}
