package pipeline_test

import (
	"context"
	"encoding/json"
	"image/gif"
	"os"
	"path/filepath"
	"testing"

	"elpanteon/engine/internal/encoder"
	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/mozart"
	"elpanteon/engine/internal/pavarotti"
	"elpanteon/engine/internal/pipeline"
	"elpanteon/engine/internal/pixar"
	"elpanteon/engine/internal/testutil"
)

func TestPipelineRunsEndToEndAndReusesArtifacts(t *testing.T) {
	out := t.TempDir()
	config := pipeline.Config{
		OutDir:      out,
		ClipSeconds: 0.5,
		FPS:         4,
		Width:       160,
		Height:      90,
		Renderer:    pixar.NewRaster(),
		Encoder:     encoder.GIFEncoder{},
		Synth:       pavarotti.ToneSynth{},
		Mixer:       mozart.WAVMixer{},
		SynthName:   "tone",
		Voice:       "test",
	}
	service, err := pipeline.New(config)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	first, err := service.Run(context.Background(), testutil.Case())
	if err != nil {
		t.Fatalf("primer Run(): %v", err)
	}
	if err := first.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	if len(first.Clips) != 9 {
		t.Fatalf("clips = %d, want 9", len(first.Clips))
	}
	if first.FrameCount != 18 {
		t.Fatalf("frames = %d, want 18", first.FrameCount)
	}
	for _, output := range first.Outputs {
		if _, err := os.Stat(output.Path); err != nil {
			t.Fatalf("output %s: %v", output.Path, err)
		}
	}
	for _, clip := range first.Clips {
		if clip.Reused {
			t.Fatalf("%s no debía reutilizarse en la primera ejecución", clip.ID)
		}
	}

	manifestPath := filepath.Join(out, "run_manifest.json")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile(): %v", err)
	}
	var decoded contract.RunManifest
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("Unmarshal(): %v", err)
	}

	master := filepath.Join(out, "master.gif")
	file, err := os.Open(master)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	animation, err := gif.DecodeAll(file)
	file.Close()
	if err != nil {
		t.Fatalf("DecodeAll(): %v", err)
	}
	if len(animation.Image) != 18 {
		t.Fatalf("gif frames = %d, want 18", len(animation.Image))
	}

	second, err := service.Run(context.Background(), testutil.Case())
	if err != nil {
		t.Fatalf("segundo Run(): %v", err)
	}
	for _, clip := range second.Clips {
		if !clip.Reused {
			t.Fatalf("%s debía reutilizarse en la segunda ejecución", clip.ID)
		}
	}
}
