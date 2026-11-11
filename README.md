# El Panteón

Motor de renderizado y orquestación audiovisual para producir casos de ingeniería
verificables. El pipeline completo está implementado en Go y funciona offline con
backends deterministas; Chromium, FFmpeg y un TTS HTTP se conectan por adaptadores.

## Inicio rápido

Desde `C:\Proyectos\panteon`:

```powershell
# Pipeline completo desde el fixture incluido
go run ./engine/cmd/panteon demo --out out/demo

# Solo Galileo: fuente -> engineering_case.json
go run ./engine/cmd/panteon galileo run --file engine/internal/galileo/testdata/postmortem.md --out out/cases

# Producir desde un caso ya extraído
go run ./engine/cmd/panteon produce --case out/cases/sha256-....json --out out/run

# Enriquecer con un repositorio real de GitHub
go run ./engine/cmd/panteon produce --case caso.json --out out/run --atlas github --repo golang/example

# UI web embebida (dashboard, revisión y renders)
go run ./engine/cmd/panteon serve --workspace out/web --addr 127.0.0.1:8080

# FFmpeg portable (solo la primera vez)
powershell -ExecutionPolicy Bypass -File .\scripts\install-ffmpeg.ps1
```

Resultado con FFmpeg local (o en el `PATH`):

```text
out/demo/run/
├── dossier_tecnico.json
├── director_cut.json
├── run_manifest.json
├── audio/master.wav
├── master.mp4
├── html/clip-01.html ...
└── clips/clip-01/frames/frame_00000.png ...
```

Sin FFmpeg, `--encoder auto` produce `master.gif` + `master.wav`.

## Arquitectura

```text
engineering_case.json
        │
     Sócrates  (gate de rigor)
        │
      Atlas    (dossier técnico)
        │
    Cervantes  (escenas y clips)
        │
   ┌────┴─────────────┐
Pavarotti          Da Vinci
(TTS)              (HTML procedural)
   │                   │
   │                Pixar (frames)
   │                   │
   └──── Mozart ───────┘
        (mezcla WAV)
             │
         Encoder
        (GIF/MP4)
```

| Fase | Paquete | Responsabilidad |
|---|---|---|
| Galileo | `engine/internal/galileo` | Fuente → caso con Tríada + Anti-Hype |
| Sócrates | `engine/internal/socrates` | Veto de rigor antes de render |
| Atlas | `engine/internal/atlas` | Caso → dossier técnico (determinista o GitHub) |
| Cervantes | `engine/internal/cervantes` | Dossier → escenas y clips |
| Da Vinci | `engine/internal/davinci` | Clip → HTML procedural |
| Pavarotti | `engine/internal/pavarotti` | Texto → WAV (tono offline o HTTP) |
| Pixar | `engine/internal/pixar` | HTML/clip → frames PNG deterministas |
| Mozart | `engine/internal/mozart` | Locuciones + música + ducking |
| Encoder | `engine/internal/encoder` | Frames → GIF o MP4 |
| Pipeline | `engine/internal/pipeline` | Orquestación, caché y manifiesto |

## Backends

| Fase | Offline (por defecto) | Real |
|---|---|---|
| Render | `--renderer raster` (Go puro) | `--renderer chromium` (Chrome/Edge) |
| TTS | `--tts tone` | `--tts http --tts-url https://...` |
| Video | `--encoder gif` | `--encoder ffmpeg` |
| Atlas | `--atlas deterministic` | `--atlas github --repo owner/name` |
| Música | Pad sintético en Go | Reemplazable en `mozart.Mixer` |

## All-in-one con Docker

```bash
cp .env.example .env
docker compose up --build
```

Levanta `panteon` (UI/API en `:8080`) y `pavarotti` (Chatterbox Multilingual TTS en
`:8000`, con UI Gradio en `/ui` y alias en `:7860`). El contenedor de Panteón recibe
`PANTEON_TTS_URL=http://pavarotti:8000`. Detalles, Easypanel y GPU en
[`docs/DEPLOY.md`](docs/DEPLOY.md).

## UI web

```powershell
go run ./engine/cmd/panteon serve --workspace out/web --addr 127.0.0.1:8080
```

Abre `http://127.0.0.1:8080`. Incluye dashboard, alta de casos desde URL/texto,
revisión Anti-Hype, dossier/guion/runs, lanzador de renders y progreso en vivo por SSE.
La UI va embebida en el binario: no requiere Node ni un servidor aparte.

## FFmpeg portable

`scripts/install-ffmpeg.ps1` descarga FFmpeg dentro de `tools/ffmpeg` sin tocar el
sistema. `--encoder auto` lo detecta automáticamente. `tools/` está en `.gitignore`.

## Documentación

- `docs/GALILEO.md`: contrato de Galileo, reglas Anti-Hype y formato de fuente.
- `docs/PIPELINE.md`: fases, contratos, CLI, caché, límites y siguientes pasos.
- `shared/contracts/`: JSON Schemas y ejemplos de los cuatro artefactos.

## Verificación

```powershell
gofmt -w engine
go vet ./...
go test ./...
```
