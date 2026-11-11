# Pavarotti TTS (Chatterbox Multilingual)

Microservicio HTTP de síntesis de voz para El Panteón, basado en
`ChatterboxMultilingualTTS` de Chatterbox. Incluye:

- API compatible con `engine/internal/pavarotti.HTTPSynth`.
- UI Gradio para clonar voz desde un audio de referencia.
- Español por defecto, con selector de idioma.

## Puertos

| Puerto | Contenido |
|---:|---|
| 8000 | API `/synthesize`, `/health`, `/voices` y UI `/ui` |
| 7860 | Alias del anterior en Compose para la UI Gradio |

Con `docker compose`, la UI queda en:

```text
http://localhost:8000/ui
http://localhost:7860/ui
```

## Endpoints

```text
GET  /health       -> {ok, model, device, language, loaded}
GET  /voices       -> voces de referencia disponibles
POST /synthesize   -> {text, voice, language, format:"wav"} -> audio/wav
```

Ejemplo:

```bash
curl -s -X POST http://localhost:8000/synthesize \
  -H "Content-Type: application/json" \
  -d '{"text":"Hola desde El Panteón","voice":"default","language":"es","format":"wav"}' \
  -o prueba.wav
```

## Voces

Dos formas de usar una voz:

1. **API**: coloca `voices/narrador.wav` y envía `"voice":"narrador"`.
2. **Gradio**: sube un audio de referencia de 6–10 s directamente en `/ui`.

Sin referencia se usa la voz base del modelo.

## Docker

```bash
docker build -t panteon-pavarotti .
docker run --rm -p 8000:8000 -p 7860:8000 \
  -v "$PWD/voices:/voices" \
  -v panteon-chatterbox-models:/models \
  -e CHATTERBOX_LANGUAGE=es \
  panteon-pavarotti
```

La primera ejecución descarga el modelo multilingüe en `/models`. En CPU el
arranque puede tardar varios minutos y el contenedor necesita varios GB de RAM.

## GPU

El Dockerfile instala PyTorch CPU por defecto. Para GPU:

```bash
docker build \
  --build-arg TORCH_INDEX_URL=https://download.pytorch.org/whl/cu124 \
  -t panteon-pavarotti .
```

Y ejecuta con `CHATTERBOX_DEVICE=cuda` y acceso a la GPU.
