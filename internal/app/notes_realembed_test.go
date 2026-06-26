//go:build integration

package app

import (
	"context"
	"os"
	"strings"
	"testing"

	"gix/internal/embed"
)

// TestCaptureFindWithRealEmbedder exercises Capture -> Find through the real
// ONNX embedder (fake AI for formatting), proving the semantic pipeline end to
// end: a query ranks on-topic notes above an unrelated one. Gated like the embed
// integration test:
//
//	GIX_ONNXRUNTIME_LIB  path to the onnxruntime shared library
//	GIX_TEST_MODELS_DIR  dir containing multilingual-e5-small/{model_quantized.onnx,tokenizer.json}
//
// Run: go test -tags integration -run RealEmbedder ./internal/app/
func TestCaptureFindWithRealEmbedder(t *testing.T) {
	lib := os.Getenv("GIX_ONNXRUNTIME_LIB")
	models := os.Getenv("GIX_TEST_MODELS_DIR")
	if lib == "" || models == "" {
		t.Skip("set GIX_ONNXRUNTIME_LIB and GIX_TEST_MODELS_DIR to run")
	}

	e, err := embed.Open(context.Background(), models)
	if err != nil {
		t.Fatalf("embed.Open: %v", err)
	}
	defer e.Close()

	d := notesTestDB(t)
	t.Setenv("AppData", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "k")

	// The fake AI returns one formatted note per capture, in order.
	fake := &fakeCompleter{responses: []string{
		`{"title":"Carro","content":"o motor do carro está fazendo um barulho estranho","tags":["carro"]}`,
		`{"title":"Oficina","content":"levei o veículo na oficina por causa do ruído","tags":["manutenção"]}`,
		`{"title":"Mercado","content":"comprei pão e leite no mercado hoje","tags":["compras"]}`,
	}}
	svc := NewNotesService(NewConfigService(), d, func(string) Completer { return fake })
	svc.setEmbedder(e)

	for _, text := range []string{"barulho no carro", "oficina do carro", "compras do mercado"} {
		if res, err := svc.Capture(text); err != nil || res.Status != "created" {
			t.Fatalf("Capture(%q) = %+v, err %v", text, res, err)
		}
	}

	results, err := svc.Find("problema de barulho no carro")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no results")
	}
	t.Logf("ranking:")
	rank := map[string]int{}
	for i, r := range results {
		t.Logf("  #%d %s (score %.4f) tags=%v", i, r.Title, r.Score, r.Tags)
		rank[r.Title] = i
	}

	// "Carro" matches both full-text (barulho, carro) and the vector, so RRF puts
	// it first.
	if results[0].Title != "Carro" {
		t.Fatalf("expected 'Carro' first, got %q (ranking %v)", results[0].Title, rank)
	}
	// "Oficina" shares no surviving query term (its match is "ruído"/"veículo"),
	// so its presence proves the semantic half of the search is contributing.
	if _, ok := rank["Oficina"]; !ok {
		t.Fatalf("semantic-only note 'Oficina' missing from results: %v", rank)
	}
	// Sanity: stored content survived AI formatting.
	if !strings.Contains(results[0].Content, "carro") {
		t.Fatalf("unexpected top content: %q", results[0].Content)
	}
}
