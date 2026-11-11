# El Panteón — Pipeline completo

Este documento describe el vertical slice implementado: de una fuente primaria a un
archivo final, pasando por todas las fases del apéndice arquitectónico.

## Estado

- **Funciona de principio a fin** en Windows/Linux/macOS sin servicios externos.
- **Galileo** extrae la Tríada y aplica Anti-Hype.
- **Sócrates** veta casos sin rigor antes de gastar render.
- **Atlas** construye un dossier técnico determinista o desde **GitHub real**
  (metadatos, README, código e issues).
- **Cervantes** genera 4 escenas y 9 clips; 10 si Atlas aporta código real.
- **Da Vinci** compone HTML procedural por clip.
- **Pavarotti** sintetiza WAV (tono offline o HTTP real).
- **Pixar** renderiza frames deterministas con Go puro o Chromium.
- **Mozart** mezcla locuciones, música y ducking en un WAV.
- **Encoder** produce MP4 si FFmpeg está disponible; si no, GIF animado.
- **Caché** por clip: la segunda ejecución reutiliza audio y frames.

## Un comando

```powershell
go run ./engine/cmd/panteon demo --out out/demo
```

Salida real del fixture incluido (con FFmpeg portable):

```text
Galileo: Incident: connection pool exhaustion
Veredicto: accept (score 100)
Caso: out\demo\cases\sha256-....json

Pipeline: Incident: connection pool exhaustion
Rigor: 100/100 (aprobado=true)
Clips: 9 · Frames: 270 · 640x360 @ 12fps
Salidas:
  [dossier] out\demo\run\dossier_tecnico.json
  [director_cut] out\demo\run\director_cut.json
  [audio] out\demo\run\audio\master.wav
  [video] out\demo\run\master.mp4
```

MP4 validado con `ffprobe`: H.264 + AAC, 640×360, 12 fps, 22.5 s, ~400 KB.
Si FFmpeg no está disponible, la misma ejecución termina en `master.gif`.

## Comandos por fase

```powershell
# 1. Galileo
go run ./engine/cmd/panteon galileo run --file fuente.md --out out/cases

# 2. Pipeline completo desde el caso
go run ./engine/cmd/panteon produce --case out/cases/sha256-....json --out out/run

# 3. Render real con Chromium
go run ./engine/cmd/panteon produce --case caso.json --out out/run --renderer chromium --chrome "C:\Program Files\Google\Chrome\Application\chrome.exe"

# 4. MP4 (requiere ffmpeg en PATH)
go run ./engine/cmd/panteon produce --case caso.json --out out/run --encoder ffmpeg

# 5. TTS HTTP tipo Chatterbox
go run ./engine/cmd/panteon produce --case caso.json --out out/run --tts http --tts-url http://localhost:8000
# 6. Atlas con un repositorio real de GitHub
go run ./engine/cmd/panteon produce --case caso.json --out out/run --atlas github --repo golang/example

# 7. FFmpeg portable (una sola vez)
powershell -ExecutionPolicy Bypass -File .\scripts\install-ffmpeg.ps1
```

### Flags de `produce` / `demo`

| Flag | Default | Descripción |
|---|---|---|
| `--out` | `out/run` / `out/demo` | Directorio de artefactos |
| `--clip-seconds` | `2.5` | Duración base de cada clip |
| `--fps` | `12` | Frames por segundo |
| `--width`, `--height` | `640x360` | Canvas de render |
| `--renderer` | `raster` | `raster` (Go) o `chromium` |
| `--chrome` | autodetectado | Ruta a Chrome/Edge |
| `--encoder` | `auto` | `auto`, `gif` o `ffmpeg` |
| `--tts` | `tone` | `tone` u `http` |
| `--tts-url` | — | URL base del TTS para `--tts http` |
| `--voice` | `default` | Voz enviada al TTS |
| `--atlas` | `deterministic` | `deterministic` o `github` |
| `--repo` | inferido de `source.url` | Repositorio `owner/name` |
| `--ref` | rama por defecto | Rama, tag o commit |
| `--github-token` | `GITHUB_TOKEN` | Token para subir el rate limit |
| `--allow-review` | `false` | Permite producir casos con veredicto `review` |
| `--clip-copies` | `1` | Repite la secuencia N veces (pruebas de carga) |
| `--timeout` | `3h` | Timeout total del pipeline |
| `--keep-awake` | `true` | Evita la suspensión del sistema en Windows |

## Artefactos

```text
out/run/
├── dossier_tecnico.json          # Atlas
├── director_cut.json             # Cervantes (duraciones ajustadas al audio)
├── run_manifest.json             # Pipeline (rigor, clips, caché, salidas)
├── audio/
│   ├── clip-01.wav ...           # Pavarotti
│   └── master.wav                # Mozart
├── html/
│   └── clip-01.html ...          # Da Vinci
├── clips/
│   └── clip-01/
│       ├── render.key            # Clave de caché
│       └── frames/frame_00000.png ...
└── master.mp4 | master.gif      # Encoder (MP4 si hay FFmpeg)
```

## Atlas GitHub

`--atlas github` consulta la API pública de GitHub:

1. Metadatos del repositorio: nombre, descripción, lenguaje y estrellas.
2. Árbol recursivo; selecciona hasta 3 archivos de código (ignora `vendor`,
   `node_modules`, `dist`, `build` y binarios).
3. README y fragmentos de código reales (límite de 8 KB por archivo).
4. Hasta 5 issues recientes, excluyendo pull requests.

Estos datos se añaden al dossier y Cervantes produce un clip extra con código real.
La API tiene rate limit; usa `--github-token` o `GITHUB_TOKEN` para subirlo.

## Contratos

| Artefacto | Schema | Ejemplo |
|---|---|---|
| `engineering_case.json` | `shared/contracts/engineering_case.schema.json` | `engineering_case.example.json` |
| `dossier_tecnico.json` | `shared/contracts/dossier_tecnico.schema.json` | `dossier_tecnico.example.json` |
| `director_cut.json` | `shared/contracts/director_cut.schema.json` | `director_cut.example.json` |
| `run_manifest.json` | `shared/contracts/run_manifest.schema.json` | `run_manifest.example.json` |

Los ejemplos se validan en `contract/pipeline_examples_test.go`.

## Rendimiento medido

Prueba real: 1920×1080, 60 fps, 909 frames, 15 s de vídeo.

| Renderer | ms/frame | FPS efectivos | 12 min estimados |
|---|---:|---:|---:|
| Raster (Go puro) | 96.4 | 10.38 | ~69 min |
| Chromium (proceso/frame) | 2303.7 | 0.43 | ~27.6 h |

Prueba completa ejecutada: 36 clips × 20 s, 43.200 frames, 1920×1080@60,
`D:\panteon-full12\master.mp4`, **720,000 s**, 23,47 MB, H.264+AAC.
Render de frames ~54 min; codificación FFmpeg ~5 h. El siguiente cuello de botella
es FFmpeg (preset por defecto), no el raster.

Detalles y comandos reproducibles en `benchmarks/BENCHMARK.md`.

## Render completo de 12 minutos

Para pruebas de carga se puede repetir la secuencia con `--clip-copies`:

```powershell
.\scripts\render-full.ps1
# Equivalente a:
.\panteon.exe produce --case caso.json --out D:\panteon-full12 `
  --clip-seconds 20 --clip-copies 4 `
  --fps 60 --width 1920 --height 1080 `
  --encoder ffmpeg --renderer raster `
  --timeout 24h --keep-awake
```

Esto produce 36 clips atómicos de 20 s = **12 minutos**, 43.200 frames a 1080p60.
`--keep-awake` usa `SetThreadExecutionState` para que Windows no suspenda el equipo.
Si el equipo duerme de todos modos, el timeout de 24 h evita que el proceso muera y
la caché permite reanudar.

## Ingesta de casos

Un caso puede entrar de tres formas:

1. **UI**: `Nuevo caso` → URL o texto → Galileo.
2. **API**: `POST /api/cases`.
3. **Watcher**: página `Fuentes` con RSS/Atom o GitHub issues.

El watcher revisa cada minuto las fuentes vencidas, deduplica, ejecuta Galileo y
guarda el caso. Los resultados se registran como eventos (`created`, `skipped`,
`error`) y se pueden notificar por webhook o Telegram:

```env
PANTEON_NOTIFY_WEBHOOK=https://discord.com/api/webhooks/...
PANTEON_TELEGRAM_TOKEN=...
PANTEON_TELEGRAM_CHAT_ID=...
```

Limitación: Galileo necesita encabezados reconocibles (Symptom/Cause/Solution o sus
variantes). Si un feed trae prosa sin esa estructura, el evento queda como `error`
y no se crea el caso.

El dashboard se refresca cada 8 segundos y muestra un aviso cuando aparece un caso
nuevo o termina un render.

## UI embebida

```powershell
.\panteon.exe serve --workspace out\web --addr 127.0.0.1:8080
```

Abre `http://127.0.0.1:8080`. La UI está embebida en el binario (sin Node) y ofrece:

- **Dashboard**: casos, jobs, runs y capacidades (FFmpeg/Chromium).
- **Nuevo caso**: URL o texto → Galileo → revisión de la Tríada, evidencia y Anti-Hype.
- **Revisión**: aprobar/rechazar con nota; un caso rechazado no se puede renderizar.
- **Caso**: dossier, guion, runs y artefactos.
- **Render**: configuración, cola en background y progreso en vivo por SSE.
- **Artefactos**: vídeo/audio/JSON y previsualización de clips HTML.

### API principal

| Método | Ruta | Función |
|---|---|---|
| GET | `/api/summary` | Estado y capacidades |
| GET/POST | `/api/cases` | Listar / crear caso |
| GET | `/api/cases/{id}` | Caso + review + runs |
| POST | `/api/cases/{id}/review` | Aprobar/rechazar |
| POST | `/api/cases/{id}/render` | Crear job |
| GET | `/api/jobs` | Listar jobs |
| GET | `/api/jobs/{id}` | Estado del job |
| GET | `/api/jobs/{id}/events` | SSE de progreso |
| GET | `/api/file?path=...` | Servir artefacto del workspace |

Seguridad: escucha en `127.0.0.1` por defecto, paths confinados al workspace y
límite de 1 MB en los cuerpos JSON. Para renders largos usa un workspace en `D:`.

## Reloj determinista

- **Raster**: el frame `i` se dibuja con `progress = i/(n-1)`; no hay reloj de pared.
- **Chromium**: Da Vinci inyecta `window.setFrame(frame, total)` y Pixar pasa
  `?frame=i&total=n`. Chrome captura con `--virtual-time-budget`, por lo que el
  resultado no depende de la velocidad de la máquina.
- El renderer Chromium captura a un mínimo de 500 px de ancho y reescala al tamaño
  pedido, porque Chrome en Windows no respeta ventanas menores.

## Caché e idempotencia

- Audio: `SHA-256(tts || voice || narración || duración)` + sidecar `.key`.
- Frames: `SHA-256(render || renderer || versión || WxH@FPS || clip JSON || hash HTML)`.
- Si la clave coincide y los frames existen, el clip se reutiliza y el manifiesto
  marca `"reused": true`.
- Cambiar renderer, versión, resolución, fps o guion invalida la caché del clip.

## Límites actuales (honestos)

1. **GIF no lleva audio embebido.** Si no hay FFmpeg se emiten `master.gif` +
   `master.wav`; con FFmpeg se emite un único `master.mp4`.
2. **FFmpeg se descarga aparte** con `scripts/install-ffmpeg.ps1` en `tools/`
   (ignorado por Git). No se instala a nivel de sistema.
3. **El TTS offline produce tonos, no voz.** Sirve para validar duración y mezcla.
   El adaptador HTTP espera un servicio que devuelva WAV.
4. **Chromium es lento**: aproximadamente 1–3 s por frame por el arranque del
   navegador. El raster es el modo por defecto. Para 60 fps reales conviene un
   cliente CDP persistente (go-rod/chromedp) en lugar de un proceso por frame.
5. **Atlas GitHub es de solo lectura y limitado**: metadatos, hasta 3 archivos de
   código, README y 5 issues. No ejecuta benchmarks ni clona el repositorio.
6. **Cervantes es heurístico**, no hay LLM de guion. Genera 4 escenas y 9–10 clips.
7. **SFX declarados pero no mezclados**; Mozart añade un click de transición.
8. **La música es un pad sintético**, no assets con licencia.
9. **`metrics` y `entities`** existen en el contrato pero el extractor determinista
   aún no los puebla.
10. **No hay Docker Compose ni servicio Pavarotti**; solo el cliente HTTP.

## Siguientes hitos

1. `extract.LLM`: extracción con JSON Schema, citas obligatorias y caché por prompt.
2. Pixar CDP persistente para 1080p60 sin arrancar Chrome por frame.
3. FFmpeg por clip + `concat demuxer` en stream copy (el diseño original).
4. Assets reales: música con licencia, SFX y tipografías.
5. Docker Compose con `pavarotti`, `engine` y un render worker.
6. Tests de contrato con un validador JSON Schema en CI.
7. Atlas autenticado con `GITHUB_TOKEN`, caché de respuestas y métricas de rate limit.
