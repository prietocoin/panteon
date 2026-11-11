package pixar

import (
	"testing"

	"elpanteon/engine/internal/galileo/contract"
)

func TestActiveBeat(t *testing.T) {
	beats := []contract.Beat{
		{Start: 0, End: 0.3, Kind: "title", Title: "A"},
		{Start: 0.3, End: 0.7, Kind: "cards", Title: "B"},
		{Start: 0.7, End: 1, Kind: "code", Title: "C"},
	}
	if got := activeBeat(beats, 0.1).Title; got != "A" {
		t.Fatalf("beat = %q", got)
	}
	if got := activeBeat(beats, 0.5).Title; got != "B" {
		t.Fatalf("beat = %q", got)
	}
	if got := activeBeat(beats, 1).Title; got != "C" {
		t.Fatalf("beat = %q", got)
	}
}
