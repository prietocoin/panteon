# Despliegue all-in-one

El repo contiene dos formas de despliegue:

1. **Un solo servicio** con `Dockerfile`: Go + Chatterbox en el mismo contenedor.
2. **Dos servicios** con `docker-compose.yml`: Panteón y Pavarotti separados.

## Un solo servicio (recomendado para Easypanel)

En el servicio `panteon`:

```text
Build:            Dockerfile
Dockerfile path:  Dockerfile
Puerto:           8080
Volúmenes:        /workspace, /models y /voices
```

Variables opcionales:

```env
CHATTERBOX_DEVICE=cpu
CHATTERBOX_LANGUAGE=es
TORCH_THREADS=4
PANTEON_LLM_BASE_URL=...
PANTEON_LLM_MODEL=...
PANTEON_LLM_API_KEY=...
PANTEON_NOTIFY_WEBHOOK=...
```

El contenedor arranca:

```text
Chatterbox TTS  -> 127.0.0.1:8000 (interno)
Panteón        -> 0.0.0.0:8080  (único puerto expuesto)
```

`PANTEON_TTS_URL` ya viene configurada a `http://127.0.0.1:8000`. La UI mostrará
**TTS ON** y el render puede usar TTS `http` con voz.

Después de desplegar esta versión, **elimina el servicio `chatterbox_tts`** de
Easypanel. Ya no es necesario.

Advertencias:

- La imagen es grande (Python + PyTorch + Chatterbox + FFmpeg).
- El primer arranque descarga el modelo en `/models` y puede tardar varios minutos.
- En CPU necesita varios GB de RAM; para producción considera GPU.
- El healthcheck de Docker solo comprueba Panteón; espera a que el TTS termine de cargar
  antes del primer render.

Con Docker Compose:

```bash
docker compose -f docker-compose.allinone.yml up --build
```

## Dos servicios

| Servicio | Puerto | Rol |
|---|---:|---|
| `panteon` | 8080 | Motor Go, API y UI embebida |
| `pavarotti` | 8000 / 7860 | Chatterbox Multilingual TTS (FastAPI + Gradio) |

En este modo, `panteon` usa `Dockerfile.panteon` (solo Go) y `pavarotti` usa
`services/pavarotti/Dockerfile`.

La UI Gradio de clonación de voz está en `http://localhost:8000/ui` y también en
`http://localhost:7860/ui` gracias al alias de Compose. El API estable usado por el
motor es `http://pavarotti:8000/synthesize`.

## Docker Compose

```bash
cp .env.example .env
docker compose up --build
```

- UI: `http://localhost:8080`
- TTS API: `http://localhost:8000/health`
- TTS UI (Gradio): `http://localhost:8000/ui` o `http://localhost:7860/ui`
- Workspace persistente: volumen `panteon_workspace`
- Modelo Chatterbox: volumen `chatterbox_models`

La primera construcción es pesada (PyTorch CPU + modelo Chatterbox). El primer
arranque de Pavarotti puede tardar varios minutos mientras descarga el modelo.

## Flujo dentro de la UI

1. Crear un caso desde URL o texto.
2. Revisar la tríada y aprobar/rechazar.
3. Pestaña **Render**:
   - TTS: `http` (la URL se toma de `PANTEON_TTS_URL`, por defecto
     `http://pavarotti:8000` dentro de Compose).
   - Voz: `default` o el nombre de un `.wav` en `services/pavarotti/voices/`.
4. Lanzar el render y seguir el progreso por SSE.

## Easypanel (all-in-one)

En el servicio `panteon`:

- Build: **Dockerfile**
- Dockerfile path: `Dockerfile`
- Puerto: `8080`
- Volúmenes: `/workspace`, `/models` y `/voices`

No uses Nixpacks para esta configuración: su build automático de Go no incluye Python
ni Chatterbox, por lo que el chip quedaría siempre en **TTS OFF**. El `Dockerfile`
raíz sí incluye ambos procesos.

`PANTEON_SEED=false` ya viene definido en la imagen. Cámbialo a `true` si quieres un
caso demo en el primer despliegue.

Si prefieres el modo de dos servicios, el servicio `panteon` usa
`Dockerfile.panteon` y el TTS se despliega aparte con `services/pavarotti/Dockerfile`.
En ese caso configura:

```env
PANTEON_TTS_URL=http://<nombre-interno-del-servicio>:8000
```

## Guionista LLM y voz

El modo escenas puede usar un LLM para escribir ~2.000 palabras y repartirlas en
6–8 bloques de duración variable. Funciona con cualquier endpoint OpenAI-compatible.

### DeepSeek (barato, recomendado para empezar)

```env
PANTEON_LLM_BASE_URL=https://api.deepseek.com/v1
PANTEON_LLM_MODEL=deepseek-chat
PANTEON_LLM_API_KEY=...
```

### Gemini Flash

```env
PANTEON_LLM_BASE_URL=https://generativelanguage.googleapis.com/v1beta/openai
PANTEON_LLM_MODEL=gemini-2.5-flash
PANTEON_LLM_API_KEY=...
```

### Kimi / Moonshot

```env
PANTEON_LLM_BASE_URL=https://api.moonshot.ai/v1
PANTEON_LLM_MODEL=moonshot-v1-8k
PANTEON_LLM_API_KEY=...
```

### Ollama local

```env
PANTEON_LLM_BASE_URL=http://host.docker.internal:11434/v1
PANTEON_LLM_MODEL=llama3.1
PANTEON_LLM_API_KEY=ollama
```

Si faltan las tres variables, Cervantes usa el guionista determinista y la UI muestra
`LLM OFF`.

### Voz real

El TTS **está en el repo pero no se despliega solo**. Para tener voz:

1. Despliega `services/pavarotti` (Chatterbox) como segundo servicio.
2. En `panteon`, define:

```env
PANTEON_TTS_URL=http://chatterbox_tts:8000
```

3. En el formulario de render elige TTS `http`.

Si `PANTEON_TTS_URL` no está definida, la UI usa `tone` (placeholder sin voz).

## Notificaciones y PWA

- PWA: la UI se instala desde el navegador del móvil (manifest + service worker).
- Alertas fuera de la app:

```env
PANTEON_NOTIFY_WEBHOOK=https://discord.com/api/webhooks/...
PANTEON_TELEGRAM_TOKEN=...
PANTEON_TELEGRAM_CHAT_ID=...
```

El watcher de fuentes guarda su estado en `/workspace/ingest`; mantén el volumen para
no perder fuentes, eventos y deduplicación.

## GPU

Por defecto se instala PyTorch CPU. Para GPU NVIDIA:

```bash
TORCH_INDEX_URL=https://download.pytorch.org/whl/cu124 \
CHATTERBOX_DEVICE=cuda \
docker compose up --build
```

Y descomenta `deploy.resources` en `docker-compose.yml` o usa `--gpus all`.

## Notas

- El contenedor `panteon` incluye FFmpeg (MP4). No incluye Chromium; usa
  `--renderer raster` o instala Chromium si necesitas el renderer de navegador.
- El workspace debe tener espacio suficiente para frames 1080p60. Usa un volumen
  en disco grande para renders largos.
- No se incluyen secretos. Usa `.env` local o variables del panel.
