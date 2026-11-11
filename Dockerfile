FROM golang:1.27-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/panteon ./engine/cmd/panteon

FROM python:3.11-slim

ENV PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    HF_HOME=/models \
    VOICE_DIR=/voices \
    CHATTERBOX_DEVICE=cpu \
    CHATTERBOX_LANGUAGE=es \
    PANTEON_TTS_URL=http://127.0.0.1:8000 \
    PANTEON_SEED=false

RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    libsndfile1 \
    git \
    curl \
    ca-certificates \
    bash \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY services/pavarotti/requirements.txt ./requirements.txt

ARG TORCH_INDEX_URL=https://download.pytorch.org/whl/cpu
RUN pip install --upgrade pip && \
    pip install torch torchaudio --index-url "${TORCH_INDEX_URL}" && \
    pip install -r requirements.txt

COPY services/pavarotti/app.py ./tts/app.py
COPY --from=build /out/panteon /usr/local/bin/panteon
COPY scripts/start-allinone.sh /usr/local/bin/start-allinone.sh
RUN chmod +x /usr/local/bin/start-allinone.sh && mkdir -p /workspace /models /voices

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=300s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/api/health || exit 1

CMD ["/usr/local/bin/start-allinone.sh"]
