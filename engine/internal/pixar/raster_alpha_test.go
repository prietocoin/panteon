package pixar

import (
	"image"
	"testing"

	"elpanteon/engine/internal/galileo/contract"
)

func TestRasterTextWithAlphaKeepsColor(t *testing.T) {
	r := NewRaster()
	if err := r.init(); err != nil {
		t.Fatal(err)
	}

	check := func(name, text string, width, height, size int, alpha uint8) {
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				img.SetRGBA(x, y, backgroundTop)
			}
		}
		face := r.face(r.bold, float64(size))
		drawCentered(img, face, height/2, text, withAlpha(textColor, alpha), width)

		bright := 0
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				c := img.RGBAAt(x, y)
				if c.R > 200 && c.G > 200 && c.B < 50 {
					t.Fatalf("%s: texto con canal azul inválido: %+v", name, c)
				}
				if c.R > 180 && c.G > 180 && c.B > 180 {
					bright++
				}
			}
		}
		if bright == 0 {
			t.Fatalf("%s: no se dibujó texto claro", name)
		}
	}

	check("small-alpha", "La causa raíz", 400, 225, 21, 224)
	check("large-alpha", "La causa raíz", 1920, 1080, 102, 224)
	check("ascii-alpha", "Incident: connection pool", 1920, 1080, 102, 224)

	clip := contract.Clip{ID: "x", DurationSec: 1, Visual: contract.Visual{Kind: "title", Title: "La causa raíz"}}
	frame := r.drawFrame(clip, 640, 360, 0.5)
	invalid := 0
	for y := 0; y < 360; y++ {
		for x := 0; x < 640; x++ {
			c := frame.RGBAAt(x, y)
			if c.R > 200 && c.G > 200 && c.B < 50 {
				invalid++
			}
		}
	}
	if invalid > 0 {
		t.Fatalf("drawFrame produjo %d píxeles de color inválido", invalid)
	}
}
