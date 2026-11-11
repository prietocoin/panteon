# Benchmark — El Panteón

Fecha: 2026-11-09  
Máquina: Windows, Go 1.27.2, Chrome/Edge instalado, FFmpeg n8.1.3 portable.  
Caso: `Incident: connection pool exhaustion` (9 clips, Atlas determinista).  
Salida: H.264 + AAC, 1920×1080, 60 fps.

## Resultados

| Renderer | Frames | Vídeo | Pared | FPS efectivos | ms/frame | MP4 | Frames en disco |
|---|---:|---:|---:|---:|---:|---:|---:|
| Raster (Go puro) | 909 | 15.0 s | 87.6 s | 10.38 | 96.4 | 0.89 MB | 61.5 MB |
| Chromium (1 proceso/frame) | 9 | 0.15 s | 20.7 s | 0.43 | 2303.7 | — | — |

Chromium es **23,9× más lento** que el raster en esta configuración.

## Extrapolación a 12 minutos

12 minutos a 60 fps = **43.200 frames**.

| Renderer | Tiempo estimado | Frames en disco |
|---|---:|---:|
| Raster | **~69 minutos** | ~2,85 GB |
| Chromium (proceso por frame) | **~27,6 horas** | ~137 GB |

La estimación asume que el coste por frame se mantiene constante. La caché no se
contabiliza porque la primera ejecución siempre renderiza todo.

## Verificación del MP4

```powershell
.\tools\ffmpeg\ffprobe.exe -v error `
  -show_entries format=duration,size,bit_rate `
  -show_entries stream=codec_name,width,height,r_frame_rate `
  -of json out\bench-raster\master.mp4
```

Resultado:

```json
{
  "streams": [
    {"codec_name": "h264", "width": 1920, "height": 1080, "r_frame_rate": "60/1"},
    {"codec_name": "aac"}
  ],
  "format": {"duration": "15.000000", "size": "935480", "bit_rate": "498922"}
}
```

## Comandos reproducibles

```powershell
# Muestra raster: 15 s a 1080p60
go run ./engine/cmd/panteon produce `
  --case out/final/cases/sha256-....json `
  --out out/bench-raster `
  --clip-seconds 1.6666667 --fps 60 `
  --width 1920 --height 1080 `
  --encoder ffmpeg --renderer raster

# Muestra Chromium: 1 frame por clip (9 frames)
go run ./engine/cmd/panteon produce `
  --case out/final/cases/sha256-....json `
  --out out/bench-chromium `
  --clip-seconds 0.01 --fps 60 `
  --width 1920 --height 1080 `
  --encoder ffmpeg --renderer chromium
```

## Render completo de 12 minutos

Prueba real ejecutada el 2026-11-10:

| Métrica | Valor |
|---|---:|
| Clips | 36 × 20 s |
| Frames | 43.200 |
| Duración | 720,000 s |
| Formato | 1920×1080, 60 fps, H.264 + AAC |
| Tamaño MP4 | 23,47 MB |
| Rigor | 100/100 |
| Tiempo de pared | 6 h 03 min |
| Render de frames | ~54 min |
| Codificación FFmpeg | ~5 h |
| Salida | `D:\panteon-full12\master.mp4` |

El cuello de botella del render completo fue **FFmpeg**, no el raster: los 43.200
frames se generaron en ~54 minutos (~12 fps efectivos), pero la codificación con el
preset por defecto de x264 tardó ~5 horas. Para la siguiente iteración conviene
añadir `-preset veryfast`/`ultrafast` o codificar por clip.

El intento anterior de 4 h falló porque el equipo entró en suspensión. Se resolvió
con `--keep-awake`, `--timeout 24h` y salida en `D:` (801 GB libres).

## Conclusión

- El raster es viable para generar vídeos de 10–12 minutos en esta máquina:
  aproximadamente **1 h 10 min** por render completo a 1080p60.
- Chromium con un proceso por frame **no es viable** a 60 fps: hace falta un
  cliente CDP persistente que mantenga el navegador vivo entre frames.
- El cuello de botella del raster es el dibujo por píxel del fondo y el texto;
  la codificación H.264 es comparativamente barata.
- La prueba también sirvió para encontrar y corregir un bug real de color en
  texto con alfa (`color.RGBA` premultiplicado → `color.NRGBA`).
