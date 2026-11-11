import os
import tempfile
import threading
from contextlib import asynccontextmanager
from pathlib import Path

import torch
import torchaudio as ta
from chatterbox.tts import ChatterboxTTS
from fastapi import FastAPI, HTTPException
from fastapi.responses import Response
from pydantic import BaseModel, Field

DEVICE = os.getenv("CHATTERBOX_DEVICE", "cpu")
VOICE_DIR = Path(os.getenv("VOICE_DIR", "/voices"))
MAX_CHARS = int(os.getenv("MAX_CHARS", "5000"))
TORCH_THREADS = int(os.getenv("TORCH_THREADS", "4"))

if TORCH_THREADS > 0:
    torch.set_num_threads(TORCH_THREADS)


class SynthRequest(BaseModel):
    text: str = Field(..., min_length=1, max_length=MAX_CHARS)
    voice: str = "default"
    format: str = "wav"


@asynccontextmanager
async def lifespan(app: FastAPI):
    app.state.model = ChatterboxTTS.from_pretrained(device=DEVICE)
    app.state.lock = threading.Lock()
    yield


app = FastAPI(title="Pavarotti TTS", version="0.1.0", lifespan=lifespan)


@app.get("/health")
def health():
    return {"ok": True, "model": "chatterbox", "device": DEVICE}


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

    kwargs = {}
    if prompt:
        kwargs["audio_prompt_path"] = prompt

    model = app.state.model
    try:
        with app.state.lock, torch.inference_mode():
            wav = model.generate(request.text, **kwargs)
            with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as temporary:
                temporary_path = temporary.name
            try:
                ta.save(temporary_path, wav, model.sr)
                body = Path(temporary_path).read_bytes()
            finally:
                try:
                    os.unlink(temporary_path)
                except OSError:
                    pass
    except Exception as exc:  # noqa: BLE001 - la API devuelve el error al cliente
        raise HTTPException(status_code=500, detail=f"fallo de síntesis: {exc}") from exc

    return Response(content=body, media_type="audio/wav", headers={
        "Content-Disposition": 'inline; filename="pavarotti.wav"',
        "X-Sample-Rate": str(model.sr),
    })
