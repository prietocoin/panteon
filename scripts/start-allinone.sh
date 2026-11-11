#!/usr/bin/env bash
set -euo pipefail

uvicorn app:app --app-dir /app/tts --host 127.0.0.1 --port 8000 &
TTS_PID=$!

/usr/local/bin/panteon serve --workspace /workspace --addr 0.0.0.0:8080 &
APP_PID=$!

cleanup() {
    kill "$TTS_PID" "$APP_PID" 2>/dev/null || true
    wait "$TTS_PID" "$APP_PID" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

wait -n "$TTS_PID" "$APP_PID"
