package pixar

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"elpanteon/engine/internal/davinci"
	"elpanteon/engine/internal/galileo/contract"
)

// Options controla el render de frames.
type Options struct {
	Width      int
	Height     int
	FPS        int
	OutDir     string
	ChromePath string
}

// Renderer produce la secuencia de PNG de un clip.
type Renderer interface {
	Name() string
	Version() string
	Render(ctx context.Context, clip contract.Clip, page davinci.Page, opts Options) ([]string, error)
}

var (
	backgroundTop    = color.RGBA{R: 15, G: 23, B: 42, A: 255}
	backgroundBottom = color.RGBA{R: 6, G: 12, B: 26, A: 255}
	panelColor       = color.RGBA{R: 30, G: 41, B: 59, A: 255}
	lineColor        = color.RGBA{R: 51, G: 65, B: 85, A: 255}
	textColor        = color.RGBA{R: 248, G: 250, B: 252, A: 255}
	mutedColor       = color.RGBA{R: 148, G: 163, B: 184, A: 255}
	skyColor         = color.RGBA{R: 56, G: 189, B: 248, A: 255}
	greenColor       = color.RGBA{R: 16, G: 185, B: 129, A: 255}
	redColor         = color.RGBA{R: 244, G: 63, B: 94, A: 255}
	amberColor       = color.RGBA{R: 245, G: 158, B: 11, A: 255}
)

// RasterRenderer dibuja frames con Go puro, sin navegador.
type RasterRenderer struct {
	once    sync.Once
	regular *sfnt.Font
	bold    *sfnt.Font
	mono    *sfnt.Font
	err     error
}

// NewRaster construye el renderer offline.
func NewRaster() *RasterRenderer {
	return &RasterRenderer{}
}

// Name implementa Renderer.
func (r *RasterRenderer) Name() string { return "raster" }

// Version implementa Renderer.
func (r *RasterRenderer) Version() string { return "2" }

// Render implementa Renderer.
func (r *RasterRenderer) Render(ctx context.Context, clip contract.Clip, page davinci.Page, opts Options) ([]string, error) {
	if err := r.init(); err != nil {
		return nil, err
	}
	if opts.Width <= 0 || opts.Height <= 0 || opts.FPS <= 0 {
		return nil, fmt.Errorf("pixar: dimensiones o fps inválidos")
	}
	frames := int(math.Ceil(clip.DurationSec * float64(opts.FPS)))
	if frames < 1 {
		frames = 1
	}
	dir := filepath.Join(opts.OutDir, safeName(clip.ID), "frames")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("pixar: creando %s: %w", dir, err)
	}

	paths := make([]string, 0, frames)
	for index := 0; index < frames; index++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		progress := 1.0
		if frames > 1 {
			progress = float64(index) / float64(frames-1)
		}
		img := r.drawFrame(clip, opts.Width, opts.Height, progress)
		path := filepath.Join(dir, fmt.Sprintf("frame_%05d.png", index))
		if err := writePNG(path, img); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func (r *RasterRenderer) init() error {
	r.once.Do(func() {
		r.regular, r.err = opentype.Parse(goregular.TTF)
		if r.err != nil {
			r.err = fmt.Errorf("pixar: fuente regular: %w", r.err)
			return
		}
		r.bold, r.err = opentype.Parse(gobold.TTF)
		if r.err != nil {
			r.err = fmt.Errorf("pixar: fuente bold: %w", r.err)
			return
		}
		r.mono, r.err = opentype.Parse(gomono.TTF)
		if r.err != nil {
			r.err = fmt.Errorf("pixar: fuente mono: %w", r.err)
		}
	})
	return r.err
}

func (r *RasterRenderer) drawFrame(clip contract.Clip, width, height int, progress float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	drawGradient(img)

	accentWidth := int(float64(width) * 0.55)
	drawGradientRect(img, image.Rect(0, 0, accentWidth, 4), skyColor, greenColor)
	drawGradientRect(img, image.Rect(accentWidth, 0, width, 4), greenColor, panelColor)

	switch clip.Visual.Kind {
	case "title":
		r.drawTitle(img, clip.Visual, progress)
	case "cards":
		r.drawCards(img, clip.Visual, progress)
	case "bullets":
		r.drawBullets(img, clip.Visual, progress)
	case "code":
		r.drawCode(img, clip.Visual, progress)
	default:
		r.drawTitle(img, clip.Visual, progress)
	}

	r.drawFooter(img, clip.ID)
	drawGradientRect(
		img,
		image.Rect(0, height-5, int(float64(width)*progress), height),
		skyColor,
		greenColor,
	)
	return img
}

func (r *RasterRenderer) drawTitle(img *image.RGBA, visual contract.Visual, progress float64) {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	eyebrowFace := r.face(r.bold, float64(height)*0.026)
	kickerColor := withAlpha(skyColor, revealAlpha(progress, 0))
	drawCentered(img, eyebrowFace, int(float64(height)*0.30), strings.ToUpper("Caso de ingeniería"), kickerColor, width)

	titleFace := r.face(r.bold, float64(height)*0.095)
	lines := wrapText(titleFace, visual.Title, int(float64(width)*0.84))
	lineHeight := int(float64(height) * 0.105)
	start := height/2 - (len(lines)-1)*lineHeight/2 - int(float64(height)*0.02)
	for index, line := range lines {
		alpha := revealAlpha(progress, 1)
		drawCentered(img, titleFace, start+index*lineHeight, line, withAlpha(textColor, alpha), width)
	}

	if visual.Subtitle != "" {
		subtitleFace := r.face(r.regular, float64(height)*0.033)
		subtitleLines := wrapText(subtitleFace, visual.Subtitle, int(float64(width)*0.76))
		base := int(float64(height) * 0.67)
		for index, line := range subtitleLines {
			drawCentered(img, subtitleFace, base+index*int(float64(height)*0.05), line, withAlpha(mutedColor, revealAlpha(progress, 2)), width)
		}
	}
}

func (r *RasterRenderer) drawCards(img *image.RGBA, visual contract.Visual, progress float64) {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	padding := int(float64(width) * 0.055)
	top := int(float64(height) * 0.255)

	headerFace := r.face(r.bold, float64(height)*0.052)
	kickerFace := r.face(r.bold, float64(height)*0.022)
	drawText(img, kickerFace, padding, int(float64(height)*0.14), withAlpha(skyColor, revealAlpha(progress, 0)), strings.ToUpper("Evidencia"))
	drawText(img, headerFace, padding, int(float64(height)*0.205), withAlpha(textColor, revealAlpha(progress, 0)), visual.Title)

	if len(visual.Cards) == 0 {
		return
	}
	columns := 2
	rows := (len(visual.Cards) + columns - 1) / columns
	gap := int(float64(width) * 0.022)
	cardWidth := (width - 2*padding - gap*(columns-1)) / columns
	available := height - top - int(float64(height)*0.13)
	cardHeight := (available - gap*(rows-1)) / rows
	if cardHeight < 40 {
		cardHeight = 40
	}

	for index, card := range visual.Cards {
		column := index % columns
		row := index / columns
		x := padding + column*(cardWidth+gap)
		y := top + row*(cardHeight+gap)
		alpha := revealAlpha(progress, index+1)
		fillRect(img, image.Rect(x, y, x+cardWidth, y+cardHeight), withAlpha(panelColor, alpha))
		fillRect(img, image.Rect(x, y, x+6, y+cardHeight), withAlpha(toneColor(card.Tone), alpha))
		fillRect(img, image.Rect(x, y, x+cardWidth, y+1), withAlpha(lineColor, alpha))

		titleFace := r.face(r.bold, float64(height)*0.031)
		bodyFace := r.face(r.regular, float64(height)*0.024)
		drawText(img, titleFace, x+int(float64(width)*0.025), y+int(float64(height)*0.07), withAlpha(textColor, alpha), card.Title)
		bodyLines := wrapText(bodyFace, card.Body, cardWidth-int(float64(width)*0.05))
		lineHeight := int(float64(height) * 0.034)
		for lineIndex, line := range bodyLines {
			drawText(img, bodyFace, x+int(float64(width)*0.025), y+int(float64(height)*0.12)+lineIndex*lineHeight, withAlpha(mutedColor, alpha), line)
		}
	}
}

func (r *RasterRenderer) drawBullets(img *image.RGBA, visual contract.Visual, progress float64) {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	padding := int(float64(width) * 0.055)
	top := int(float64(height) * 0.265)

	headerFace := r.face(r.bold, float64(height)*0.052)
	kickerFace := r.face(r.bold, float64(height)*0.022)
	drawText(img, kickerFace, padding, int(float64(height)*0.14), withAlpha(skyColor, revealAlpha(progress, 0)), strings.ToUpper("Análisis"))
	drawText(img, headerFace, padding, int(float64(height)*0.205), withAlpha(textColor, revealAlpha(progress, 0)), visual.Title)

	if len(visual.Bullets) == 0 {
		return
	}
	gap := int(float64(height) * 0.025)
	available := height - top - int(float64(height)*0.13)
	itemHeight := (available - gap*(len(visual.Bullets)-1)) / len(visual.Bullets)
	if itemHeight < 36 {
		itemHeight = 36
	}
	bodyFace := r.face(r.regular, float64(height)*0.028)

	for index, bullet := range visual.Bullets {
		alpha := revealAlpha(progress, index+1)
		y := top + index*(itemHeight+gap)
		fillRect(img, image.Rect(padding, y, width-padding, y+itemHeight), withAlpha(panelColor, alpha))
		fillRect(img, image.Rect(padding, y, width-padding, y+1), withAlpha(lineColor, alpha))
		fillRect(img, image.Rect(padding, y, padding+5, y+itemHeight), withAlpha(greenColor, alpha))
		fillRect(img, image.Rect(padding+int(float64(width)*0.018), y+itemHeight/2-4, padding+int(float64(width)*0.018)+8, y+itemHeight/2+4), withAlpha(greenColor, alpha))
		lines := wrapText(bodyFace, bullet, width-2*padding-int(float64(width)*0.06))
		lineHeight := int(float64(height) * 0.038)
		startY := y + itemHeight/2 - (len(lines)-1)*lineHeight/2 + lineHeight/3
		for lineIndex, line := range lines {
			drawText(img, bodyFace, padding+int(float64(width)*0.035), startY+lineIndex*lineHeight, withAlpha(textColor, alpha), line)
		}
	}
}

func (r *RasterRenderer) drawCode(img *image.RGBA, visual contract.Visual, progress float64) {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	padding := int(float64(width) * 0.055)
	top := int(float64(height) * 0.255)

	headerFace := r.face(r.bold, float64(height)*0.052)
	kickerFace := r.face(r.bold, float64(height)*0.022)
	drawText(img, kickerFace, padding, int(float64(height)*0.14), withAlpha(skyColor, revealAlpha(progress, 0)), strings.ToUpper(visual.CodeLanguage))
	drawText(img, headerFace, padding, int(float64(height)*0.205), withAlpha(textColor, revealAlpha(progress, 0)), visual.Title)

	alpha := revealAlpha(progress, 1)
	panelTop := top
	panelBottom := height - int(float64(height)*0.11)
	fillRect(img, image.Rect(padding, panelTop, width-padding, panelBottom), withAlpha(color.RGBA{R: 2, G: 6, B: 23, A: 255}, alpha))
	fillRect(img, image.Rect(padding, panelTop, width-padding, panelTop+1), withAlpha(lineColor, alpha))

	codeFace := r.face(r.mono, float64(height)*0.024)
	lineHeight := int(float64(height) * 0.038)
	startY := panelTop + lineHeight + int(float64(height)*0.015)
	maxWidth := width - 2*padding - int(float64(width)*0.04)
	for index, line := range strings.Split(visual.Code, "\n") {
		y := startY + index*lineHeight
		if y > panelBottom-lineHeight/2 {
			break
		}
		textTone := mutedColor
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "#"):
			textTone = color.RGBA{R: 100, G: 116, B: 139, A: 255}
		case strings.HasPrefix(trimmed, "-"):
			textTone = greenColor
		case strings.Contains(line, ":"):
			textTone = skyColor
		}
		drawText(img, codeFace, padding+int(float64(width)*0.02), y, withAlpha(textTone, alpha), fitText(codeFace, line, maxWidth))
	}
}

func (r *RasterRenderer) drawFooter(img *image.RGBA, clipID string) {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	face := r.face(r.regular, float64(height)*0.017)
	drawText(img, face, int(float64(width)*0.055), height-int(float64(height)*0.035), color.RGBA{R: 100, G: 116, B: 139, A: 255}, strings.ToUpper("El Panteón · Galileo"))
	label := strings.ToUpper(clipID)
	textWidth := font.MeasureString(face, label).Ceil()
	drawText(img, face, width-int(float64(width)*0.055)-textWidth, height-int(float64(height)*0.035), color.RGBA{R: 100, G: 116, B: 139, A: 255}, label)
}

func (r *RasterRenderer) face(base *sfnt.Font, size float64) font.Face {
	if size < 8 {
		size = 8
	}
	face, err := opentype.NewFace(base, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil
	}
	return face
}

func drawGradient(img *image.RGBA) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	for y := 0; y < height; y++ {
		t := float64(y) / float64(max(height-1, 1))
		c := lerpColor(backgroundTop, backgroundBottom, t)
		offset := img.PixOffset(bounds.Min.X, bounds.Min.Y+y)
		row := img.Pix[offset : offset+width*4]
		for i := 0; i < len(row); i += 4 {
			row[i] = c.R
			row[i+1] = c.G
			row[i+2] = c.B
			row[i+3] = 255
		}
	}
}

func drawGradientRect(img *image.RGBA, rect image.Rectangle, from, to color.RGBA) {
	width := max(rect.Dx(), 1)
	for x := 0; x < rect.Dx(); x++ {
		c := lerpColor(from, to, float64(x)/float64(width))
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			img.SetRGBA(rect.Min.X+x, y, c)
		}
	}
}

func fillRect(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
	if c.A == 0 {
		return
	}
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return
	}
	if c.A == 255 {
		draw.Draw(img, rect, image.NewUniform(c), image.Point{}, draw.Src)
		return
	}
	alpha := uint32(c.A)
	inverse := 255 - alpha
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		offset := img.PixOffset(rect.Min.X, y)
		row := img.Pix[offset : offset+rect.Dx()*4]
		for i := 0; i < len(row); i += 4 {
			row[i] = uint8((uint32(c.R)*alpha + uint32(row[i])*inverse) / 255)
			row[i+1] = uint8((uint32(c.G)*alpha + uint32(row[i+1])*inverse) / 255)
			row[i+2] = uint8((uint32(c.B)*alpha + uint32(row[i+2])*inverse) / 255)
			row[i+3] = 255
		}
	}
}

func drawText(img *image.RGBA, face font.Face, x, baseline int, c color.RGBA, text string) {
	if face == nil || c.A == 0 || text == "" {
		return
	}
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}),
		Face: face,
		Dot:  fixed.P(x, baseline),
	}
	drawer.DrawString(text)
}

func drawCentered(img *image.RGBA, face font.Face, baseline int, text string, c color.RGBA, width int) {
	if face == nil {
		return
	}
	textWidth := font.MeasureString(face, text).Ceil()
	drawText(img, face, (width-textWidth)/2, baseline, c, text)
}

func wrapText(face font.Face, text string, maxWidth int) []string {
	if face == nil || maxWidth <= 0 {
		return []string{text}
	}
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			continue
		}
		line := words[0]
		for _, word := range words[1:] {
			candidate := line + " " + word
			if font.MeasureString(face, candidate).Ceil() <= maxWidth {
				line = candidate
				continue
			}
			lines = append(lines, line)
			line = word
			for font.MeasureString(face, line).Ceil() > maxWidth && len(line) > 1 {
				runes := []rune(line)
				cut := len(runes) - 1
				for cut > 1 && font.MeasureString(face, string(runes[:cut])).Ceil() > maxWidth {
					cut--
				}
				lines = append(lines, string(runes[:cut]))
				line = string(runes[cut:])
			}
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func fitText(face font.Face, text string, maxWidth int) string {
	if face == nil || font.MeasureString(face, text).Ceil() <= maxWidth {
		return text
	}
	runes := []rune(text)
	for len(runes) > 1 && font.MeasureString(face, string(runes)).Ceil() > maxWidth {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}

func toneColor(tone string) color.RGBA {
	switch tone {
	case "danger":
		return redColor
	case "success":
		return greenColor
	case "warning":
		return amberColor
	default:
		return skyColor
	}
}

func withAlpha(c color.RGBA, alpha uint8) color.RGBA {
	c.A = uint8(uint32(c.A) * uint32(alpha) / 255)
	return c
}

func revealAlpha(progress float64, index int) uint8 {
	local := (progress - float64(index)*0.08) / 0.38
	if local < 0 {
		local = 0
	}
	if local > 1 {
		local = 1
	}
	return uint8(local * 255)
}

func lerpColor(from, to color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(from.R) + (float64(to.R)-float64(from.R))*t),
		G: uint8(float64(from.G) + (float64(to.G)-float64(from.G))*t),
		B: uint8(float64(from.B) + (float64(to.B)-float64(from.B))*t),
		A: 255,
	}
}

func writePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("pixar: creando %s: %w", path, err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("pixar: codificando %s: %w", path, err)
	}
	return nil
}

func safeName(value string) string {
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, value)
	name = strings.Trim(name, "-")
	if name == "" {
		return "clip"
	}
	return name
}
