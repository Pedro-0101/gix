package embed

import (
	"math"
	"testing"
)

func TestMeanPoolMasksPadding(t *testing.T) {
	// Two tokens of a 2-dim hidden state; the second is padding (mask 0) and must
	// be ignored, so the pooled vector equals the first token.
	data := []float32{1, 2, 100, 100}
	got := meanPool(data, []int{1, 0}, 2)
	want := []float32{1, 2}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("meanPool[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestMeanPoolAverages(t *testing.T) {
	data := []float32{1, 3, 3, 1}
	got := meanPool(data, []int{1, 1}, 2)
	want := []float32{2, 2}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("meanPool[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestL2NormalizeUnitLength(t *testing.T) {
	v := []float32{3, 4}
	l2normalize(v)
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	if math.Abs(s-1) > 1e-6 {
		t.Fatalf("norm^2 = %v, want 1", s)
	}
}

func TestCosineIdentical(t *testing.T) {
	a := []float32{1, 0, 0}
	if c := Cosine(a, a); math.Abs(c-1) > 1e-6 {
		t.Fatalf("Cosine(a,a) = %v, want 1", c)
	}
	b := []float32{0, 1, 0}
	if c := Cosine(a, b); math.Abs(c) > 1e-6 {
		t.Fatalf("Cosine(a,b) = %v, want 0", c)
	}
}

func TestVectorRoundTrip(t *testing.T) {
	v := []float32{0.5, -1.25, 3.0, 0}
	got := DecodeVector(EncodeVector(v))
	if len(got) != len(v) {
		t.Fatalf("len = %d, want %d", len(got), len(v))
	}
	for i := range v {
		if got[i] != v[i] {
			t.Fatalf("round-trip[%d] = %v, want %v", i, got[i], v[i])
		}
	}
}
