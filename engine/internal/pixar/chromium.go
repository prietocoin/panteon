package pixar

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"

	"elpanteon/engine/internal/davinci"
	"elpanteon/engine/internal/galileo/contract"
)

const minChromiumWidth = 500

// ChromiumRenderer captura frames reales de Chromium usando el reloj virtual
// inyectado en el HTML (?frame=N&total=M). Es más lento que el renderer raster.
type ChromiumRenderer struct {
	Path string
}

// NewChromium construye el renderer de navegador.
func NewChromium(path string) *ChromiumRenderer {
	return &ChromiumRenderer{Path: path}
}

// Name implementa Renderer.
func (c *ChromiumRenderer) Name() string { return "chromium" }

// Version implementa Renderer.
func (c *ChromiumRenderer) Version() string { return "2" }

// Render implementa Renderer.
func (c *ChromiumRenderer) Render(ctx context.Context, clip contract.Clip, page davinci.Page, opts Options) ([]string, error) {
	browser := c.Path
	if strings.TrimSpace(browser) == "" {
		browser = FindChrome()
	}
	if browser == "" {
		return nil, errors.New("pixar: no se encontró Chrome/Chromium; usa --chrome o el renderer raster")
	}
	if opts.Width <= 0 || opts.Height <= 0 || opts.FPS <= 0 {
		return nil, errors.New("pixar: dimensiones o fps inválidos")
	}

	frames := int(clip.DurationSec*float64(opts.FPS) + 0.999)
	if frames < 1 {
		frames = 1
	}
	dir := filepath.Join(opts.OutDir, safeName(clip.ID), "frames")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("pixar: creando %s: %w", dir, err)
	}

	profile, err := os.MkdirTemp("", "elpanteon-chrome-*")
	if err != nil {
		return nil, fmt.Errorf("pixar: creando perfil temporal: %w", err)
	}
	defer os.RemoveAll(profile)

	captureWidth := opts.Width
	captureHeight := opts.Height
	if captureWidth < minChromiumWidth {
		captureWidth = minChromiumWidth
		captureHeight = int(math.Round(float64(captureWidth) * float64(opts.Height) / float64(opts.Width)))
	}

	htmlURL := ""
	if absolute, err := filepath.Abs(page.Path); err == nil {
		htmlURL = (&url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}).String()
	} else {
		return nil, fmt.Errorf("pixar: resolviendo HTML: %w", err)
	}
	paths := make([]string, 0, frames)
	for index := 0; index < frames; index++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		framePath, err := filepath.Abs(filepath.Join(dir, fmt.Sprintf("frame_%05d.png", index)))
		if err != nil {
			return nil, err
		}
		frameURL := fmt.Sprintf("%s?frame=%d&total=%d", htmlURL, index, frames)
		command := exec.CommandContext(
			ctx,
			browser,
			"--headless=new",
			"--disable-gpu",
			"--hide-scrollbars",
			"--no-first-run",
			"--no-default-browser-check",
			"--disable-extensions",
			"--force-device-scale-factor=1",
			fmt.Sprintf("--window-size=%d,%d", captureWidth, captureHeight),
			"--virtual-time-budget=350",
			"--user-data-dir="+profile,
			"--screenshot="+framePath,
			frameURL,
		)
		output, err := command.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("pixar: Chromium falló en frame %d: %w: %s", index, err, strings.TrimSpace(string(output)))
		}
		info, err := os.Stat(framePath)
		if err != nil || info.Size() == 0 {
			return nil, fmt.Errorf("pixar: Chromium no escribió el frame %d", index)
		}
		if captureWidth != opts.Width {
			if err := resizePNG(framePath, opts.Width, opts.Height); err != nil {
				return nil, err
			}
		}
		paths = append(paths, framePath)
	}
	return paths, nil
}

// FindChrome localiza Chrome, Chromium o Edge en el sistema.
func FindChrome() string {
	if candidate := strings.TrimSpace(os.Getenv("PANTEON_CHROME")); candidate != "" {
		return candidate
	}
	for _, name := range []string{"chrome", "chrome.exe", "chromium", "chromium.exe", "msedge", "msedge.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	candidates := []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`/usr/bin/google-chrome`,
		`/usr/bin/chromium`,
		`/usr/bin/chromium-browser`,
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func resizePNG(path string, width, height int) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("pixar: abriendo frame: %w", err)
	}
	source, err := png.Decode(file)
	file.Close()
	if err != nil {
		return fmt.Errorf("pixar: decodificando frame: %w", err)
	}
	target := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.ApproxBiLinear.Scale(target, target.Bounds(), source, source.Bounds(), xdraw.Over, nil)

	temporary := path + ".tmp"
	out, err := os.Create(temporary)
	if err != nil {
		return fmt.Errorf("pixar: creando frame: %w", err)
	}
	if err := png.Encode(out, target); err != nil {
		out.Close()
		_ = os.Remove(temporary)
		return fmt.Errorf("pixar: codificando frame: %w", err)
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("pixar: publicando frame: %w", err)
	}
	return nil
}
