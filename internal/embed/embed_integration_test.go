//go:build integration

package embed

import (
	"context"
	"os"
	"testing"
)

// TestEmbedIntegration runs the real model. It is gated behind the `integration`
// build tag and two env vars so it never runs (or downloads 135MB) by default:
//
//	GIX_ONNXRUNTIME_LIB  path to the onnxruntime shared library
//	GIX_TEST_MODELS_DIR  dir already containing multilingual-e5-small/{model_quantized.onnx,tokenizer.json}
func TestEmbedIntegration(t *testing.T) {
	lib := os.Getenv("GIX_ONNXRUNTIME_LIB")
	modelsDir := os.Getenv("GIX_TEST_MODELS_DIR")
	if lib == "" || modelsDir == "" {
		t.Skip("set GIX_ONNXRUNTIME_LIB and GIX_TEST_MODELS_DIR to run")
	}

	e, err := Open(context.Background(), modelsDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer e.Close()

	// Real retrieval shape: one query vs several stored notes (docs). The
	// on-topic note must rank above the off-topic ones.
	query, err := e.EmbedQuery("problema de barulho no motor do carro")
	if err != nil {
		t.Fatalf("EmbedQuery: %v", err)
	}
	if len(query) != Dim {
		t.Fatalf("dim = %d, want %d", len(query), Dim)
	}

	docs := map[string]string{
		"onTopic":    "levei o veículo na oficina por causa de um ruído estranho no motor",
		"offMarket":  "comprei pão e leite no mercado hoje de manhã",
		"offMeeting": "reunião com o time de produto na sexta às 15h",
	}
	sims := map[string]float64{}
	for name, text := range docs {
		v, err := e.EmbedDoc(text)
		if err != nil {
			t.Fatalf("EmbedDoc %s: %v", name, err)
		}
		sims[name] = Cosine(query, v)
		t.Logf("cos(query, %s) = %.4f", name, sims[name])
	}
	if sims["onTopic"] <= sims["offMarket"] || sims["onTopic"] <= sims["offMeeting"] {
		t.Fatalf("on-topic note did not rank first: %+v", sims)
	}
}
