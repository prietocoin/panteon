import os
import tempfile
import threading
from contextlib import asynccontextmanager
from pathlib import Path

import gradio as gr
import torch
import torchaudio as ta
from chatterbox.mtl_tts import ChatterboxMultilingualTTS
from fastapi import FastAPI, HTTPException
from fastapi.responses import Response
from pydantic import BaseModel, Field

DEVICE = os.getenv("CHATTERBOX_DEVICE", "cpu")
VOICE_DIR = Path(os.getenv("VOICE_DIR", "/voices"))
MAX_CHARS = int(os.getenv("MAX_CHARS", "5000"))
TORCH_THREADS = int(os.getenv("TORCH_THREADS", "4"))
DEFAULT_LANGUAGE = os.getenv("CHATTERBOX_LANGUAGE", "es")

if TORCH_THREADS > 0:
    torch.set_num_threads(TORCH_THREADS)

_model = None
_model_lock = threading.Lock()
_generation_lock = threading.Lock()


def get_model() -> ChatterboxMultilingualTTS:
    global _model
    if _model is None:
        with _model_lock:
            if _model is None:
                _model = ChatterboxMultilingualTTS.from_pretrained(device=DEVICE)
    return _model


def synth_bytes(text: str, language: str, audio_prompt_path: str | None) -> bytes:
    model = get_model()
    kwargs = {"language_id": language or DEFAULT_LANGUAGE}
    if audio_prompt_path:
        kwargs["audio_prompt_path"] = audio_prompt_path

    try:
        with _generation_lock, torch.inference_mode():
            wav = model.generate(text, **kwargs)
            with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as temporary:
                temporary_path = temporary.name
            try:
                ta.save(temporary_path, wav, model.sr)
                return Path(temporary_path).read_bytes()
            finally:
                try:
                    os.unlink(temporary_path)
                except OSError:
                    pass
    except Exception as exc:  # noqa: BLE001
        raise HTTPException(status_code=500, detail=f"fallo de síntesis: {exc}") from exc


class SynthRequest(BaseModel):
    text: str = Field(..., min_length=1, max_length=MAX_CHARS)
    voice: str = "default"
    language: str = DEFAULT_LANGUAGE
    format: str = "wav"


@asynccontextmanager
async def lifespan(_: FastAPI):
    # Pre-carga el modelo para que el primer /synthesize no agote el timeout.
    get_model()
    yield


app = FastAPI(title="Pavarotti TTS", version="0.2.0", lifespan=lifespan)


@app.get("/health")
def health():
    return {
        "ok": True,
        "model": "chatterbox-multilingual",
        "device": DEVICE,
        "language": DEFAULT_LANGUAGE,
        "loaded": _model is not None,
    }


@app.get("/voices")
def voices():
    if not VOICE_DIR.exists():
        return {"voices": []}
    return {"voices": sorted(path.stem for path in VOICE_DIR.glob("*.wav"))}


@app.post("/synthesize")
def synthesize(request: SynthRequest):
    if request.format.lower() != "wav":
        raise HTTPException(status_code=400, detail="solo se soporta format=wav")

    prompt = None
    if request.voice and request.voice != "default":
        candidate = VOICE_DIR / f"{request.voice}.wav"
        if not candidate.is_file():
            raise HTTPException(status_code=400, detail=f"voz no encontrada: {request.voice}")
        prompt = str(candidate)

    body = synth_bytes(request.text, request.language, prompt)
    model = get_model()
    return Response(
        content=body,
        media_type="audio/wav",
        headers={
            "Content-Disposition": 'inline; filename="pavarotti.wav"',
            "X-Sample-Rate": str(model.sr),
            "X-Language": request.language or DEFAULT_LANGUAGE,
        },
    )


def clonar_voz(audio_ref, texto, idioma):
    if not texto:
        return None
    body = synth_bytes(texto, idioma or DEFAULT_LANGUAGE, audio_ref or None)
    with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as output:
        output.write(body)
        return output.name


demo = gr.Interface(
    fn=clonar_voz,
    inputs=[
        gr.Audio(type="filepath", label="Audio de referencia (6-10s)"),
        gr.Textbox(lines=4, label="Texto a generar"),
        gr.Dropdown(
            choices=["es", "en", "fr", "de", "it", "pt"],
            value=DEFAULT_LANGUAGE,
            label="Idioma",
        ),
    ],
    outputs=gr.Audio(label="Voz generada"),
    title="Chatterbox Multilingual · Pavarotti",
    description="Clonación de voz y síntesis multilingüe. El API estable está en /synthesize.",
)

app = gr.mount_gradio_app(app, demo, path="/ui")
