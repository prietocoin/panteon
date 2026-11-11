FROM golang:1.27-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/panteon ./engine/cmd/panteon

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/panteon /usr/local/bin/panteon

ENV PANTEON_TTS_URL=http://pavarotti:8000
EXPOSE 8080

ENTRYPOINT ["panteon"]
CMD ["serve", "--workspace", "/workspace", "--addr", "0.0.0.0:8080"]
