package textutil_test

import (
	"testing"

	"elpanteon/engine/internal/textutil"
)

func TestSentencesKeepsDecimals(t *testing.T) {
	got := textutil.Sentences("La latencia subió a 4.2 ms. Luego bajó. ¿Y después?")
	if len(got) != 3 {
		t.Fatalf("Sentences() = %#v", got)
	}
	if got[0] != "La latencia subió a 4.2 ms." {
		t.Fatalf("primera oración = %q", got[0])
	}
}

func TestTruncateAndBullets(t *testing.T) {
	if got := textutil.Truncate("abcdef", 4); got != "abc…" {
		t.Fatalf("Truncate() = %q", got)
	}
	bullets := textutil.Bullets("Uno. Dos. Tres. Cuatro.", 3)
	if len(bullets) != 3 {
		t.Fatalf("Bullets() = %#v", bullets)
	}
}
