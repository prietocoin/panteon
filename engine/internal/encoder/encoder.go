package encoder

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color/palette"
	"image/gif"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// Encoder convierte una secuencia de PNG en el contenedor final.
type Encoder interface {
	Name() string
	Encode(ctx context.Context, frames []string, audioPath, outPath string, fps int) (string, error)
}

// Select elige el encoder por nombre.
func Select(name string) Encoder {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ffmpeg":
		return FFmpegEncoder{Binary: findFFmpeg()}
	case "gif":
		return GIFEncoder{}
	default:
		if binary := findFFmpeg(); binary != "" {
			return FFmpegEncoder{Binary: binary}
		}
		return GIFEncoder{}
	}
}

// GIFEncoder genera un GIF animado y deja el WAV como pista separada.
type GIFEncoder struct{}

// Name implementa Encoder.
func (GIFEncoder) Name() string { return "gif" }

// Encode implementa Encoder.
func (e GIFEncoder) Encode(ctx context.Context, frames []string, audioPath, outPath string, fps int) (string, error) {
	if len(frames) == 0 {
		return "", errors.New("encoder: no hay frames")
	}
	if fps <= 0 {
		return "", errors.New("encoder: fps inválido")
	}
	outPath = ensureExtension(outPath, ".gif")

	delay := 100 / fps
	if delay < 2 {
		delay = 2
	}

	animated := &gif.GIF{LoopCount: 0}
	for _, framePath := range frames {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		file, err := os.Open(framePath)
		if err != nil {
			return "", fmt.Errorf("encoder: abriendo %s: %w", framePath, err)
		}
		source, err := png.Decode(file)
		file.Close()
		if err != nil {
			return "", fmt.Errorf("encoder: decodificando %s: %w", framePath, err)
		}
		rect := image.Rect(0, 0, source.Bounds().Dx(), source.Bounds().Dy())
		paletted := image.NewPaletted(rect, palette.Plan9)
		xdraw.FloydSteinberg.Draw(paletted, rect, source, source.Bounds().Min)
		animated.Image = append(animated.Image, paletted)
		animated.Delay = append(animated.Delay, delay)
	}

	temporary := outPath + ".tmp"
	file, err := os.Create(temporary)
	if err != nil {
		return "", fmt.Errorf("encoder: creando %s: %w", temporary, err)
	}
	if err := gif.EncodeAll(file, animated); err != nil {
		file.Close()
		_ = os.Remove(temporary)
		return "", fmt.Errorf("encoder: codificando GIF: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temporary, outPath); err != nil {
		_ = os.Remove(temporary)
		return "", fmt.Errorf("encoder: publicando %s: %w", outPath, err)
	}
	return outPath, nil
}

// FFmpegEncoder emite MP4 cuando ffmpeg está instalado.
type FFmpegEncoder struct {
	Binary string
}

// Name implementa Encoder.
func (e FFmpegEncoder) Name() string { return "ffmpeg" }

// Encode implementa Encoder.
func (e FFmpegEncoder) Encode(ctx context.Context, frames []string, audioPath, outPath string, fps int) (string, error) {
	if len(frames) == 0 {
		return "", errors.New("encoder: no hay frames")
	}
	binary := e.Binary
	if binary == "" {
		binary = findFFmpeg()
	}
	if binary == "" {
		return "", errors.New("encoder: ffmpeg no está disponible")
	}
	if fps <= 0 {
		return "", errors.New("encoder: fps inválido")
	}
	outPath = ensureExtension(outPath, ".mp4")

	list, err := os.CreateTemp("", "elpanteon-frames-*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(list.Name())
	for _, framePath := range frames {
		absolute, err := filepath.Abs(framePath)
		if err != nil {
			list.Close()
			return "", err
		}
		escaped := strings.ReplaceAll(filepath.ToSlash(absolute), "'", `'\''`)
		if _, err := fmt.Fprintf(list, "file '%s'\nduration %.6f\n", escaped, 1.0/float64(fps)); err != nil {
			list.Close()
			return "", err
		}
	}
	if err := list.Close(); err != nil {
		return "", err
	}

	args := []string{
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", list.Name(),
	}
	if audioPath != "" {
		args = append(args, "-i", audioPath)
	}
	args = append(args, "-r", fmt.Sprintf("%d", fps), "-c:v", "libx264", "-pix_fmt", "yuv420p", "-movflags", "+faststart")
	if audioPath != "" {
		args = append(args, "-c:a", "aac", "-shortest")
	}
	args = append(args, outPath)

	command := exec.CommandContext(ctx, binary, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("encoder: ffmpeg falló: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return outPath, nil
}

func findFFmpeg() string {
	if configured := strings.TrimSpace(os.Getenv("PANTEON_FFMPEG")); configured != "" {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured
		}
	}

	candidates := []string{
		filepath.Join("tools", "ffmpeg", "bin", "ffmpeg.exe"),
		filepath.Join("tools", "ffmpeg", "ffmpeg.exe"),
	}
	if executable, err := os.Executable(); err == nil {
		base := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(base, "tools", "ffmpeg", "bin", "ffmpeg.exe"),
			filepath.Join(base, "..", "tools", "ffmpeg", "bin", "ffmpeg.exe"),
		)
	}
	for _, candidate := range candidates {
		if absolute, err := filepath.Abs(candidate); err == nil {
			if info, err := os.Stat(absolute); err == nil && !info.IsDir() {
				return absolute
			}
		}
	}

	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return ""
	}
	return path
}

func ensureExtension(path, extension string) string {
	if strings.EqualFold(filepath.Ext(path), extension) {
		return path
	}
	return strings.TrimSuffix(path, filepath.Ext(path)) + extension
}
