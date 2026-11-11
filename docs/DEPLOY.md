# Despliegue all-in-one

El repo contiene dos servicios:

| Servicio | Puerto | Rol |
|---|---:|---|
| `panteon` | 8080 | Motor Go, API y UI embebida |
| `pavarotti` | 8000 / 7860 | Chatterbox Multilingual TTS (FastAPI + Gradio) |

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

## Easypanel

Puedes desplegar los dos servicios por separado:

1. **panteon**: build desde el `Dockerfile` raíz, puerto 8080, volumen `/workspace`.
2. **chatterbox_tts**: build desde `services/pavarotti/Dockerfile`, puerto 8000
   (y opcionalmente 7860 para la UI Gradio), volúmenes `/models` y `/voices`.

Configura en `panteon`:

```text
PANTEON_TTS_URL=http://<nombre-interno-del-servicio>:8000
```

Si el servicio en Easypanel se llama `chatterbox_tts`, usa
`http://chatterbox_tts:8000`. La URL también se puede cambiar por render en el
formulario de la UI.

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
