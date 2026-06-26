//go:build integration

package embed

import (
	"context"
	"os"
	"testing"
)

// TestLibAutoDownload exercises ensureLib's download+extract path: with no
// LibEnvVar override and no lib next to the test binary, Open must fetch the
// official onnxruntime release and extract the shared library, then embed.
// Reuses GIX_TEST_MODELS_DIR (already holding the model) so only the ~77MB lib
// archive downloads.
//
// Run: go test -tags integration -run LibAutoDownload ./internal/embed/
func TestLibAutoDownload(t *testing.T) {
	models := os.Getenv("GIX_TEST_MODELS_DIR")
	if models == "" {
		t.Skip("set GIX_TEST_MODELS_DIR (with multilingual-e5-small/ inside) to run")
	}
	t.Setenv(LibEnvVar, "") // force the download path, ignoring any external override

	e, err := Open(context.Background(), models)
	if err != nil {
		t.Fatalf("Open (lib auto-download): %v", err)
	}
	defer e.Close()

	v, err := e.EmbedQuery("teste de download da biblioteca nativa")
	if err != nil {
		t.Fatalf("EmbedQuery: %v", err)
	}
	if len(v) != Dim {
		t.Fatalf("dim = %d, want %d", len(v), Dim)
	}
}
