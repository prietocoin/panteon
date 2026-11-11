# Pavarotti TTS (Chatterbox)

Microservicio HTTP de síntesis de voz para El Panteón. Implementa el endpoint que
consume `engine/internal/pavarotti.HTTPSynth`.

## Endpoints

```text
GET  /health       -> {ok, model, device}
GET  /voices       -> voces de referencia disponibles
POST /synthesize   -> {text, voice, format:"wav"} -> audio/wav
```

## Docker

```bash
docker build -t panteon-pavarotti .
docker run --rm -p 8000:8000 \
  -v "$PWD/voices:/voices" \
  -v panteon-chatterbox-models:/models \
  panteon-pavarotti
```

La primera ejecución descarga el modelo de Chatterbox en `/models`. Para CPU el
primer arranque puede tardar varios minutos.

## Prueba

```bash
curl -s http://localhost:8000/health
curl -s -X POST http://localhost:8000/synthesize \
  -H "Content-Type: application/json" \
  -d '{"text":"Hola desde El Panteón","voice":"default","format":"wav"}' \
  -o prueba.wav
```

## Voces

Coloca archivos `.wav` en `voices/`. El nombre del archivo es el nombre de la voz:

```text
voices/narrador.wav -> {"voice":"narrador"}
```

Sin voz, usa `default` (voz base de Chatterbox).

## GPU

El Dockerfile instala PyTorch CPU por defecto. Para GPU:

```bash
docker build \
  --build-arg TORCH_INDEX_URL=https://download.pytorch.org/whl/cu124 \
  -t panteon-pavarotti .
```

Y ejecuta con `CHATTERBOX_DEVICE=cuda` y acceso a la GPU (`--gpus all` o
`deploy.resources` en Compose).
