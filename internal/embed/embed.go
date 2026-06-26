// Package embed turns text into dense vectors for the notes' semantic search.
// It runs the multilingual-e5-small model (int8 ONNX) via onnxruntime, loaded at
// runtime through purego — so the gix code stays CGo-free; only the native
// onnxruntime shared library must ship alongside the app.
//
// The model and tokenizer are downloaded on first use (see download.go); the
// native library path is resolved by ResolveLibPath (see lib.go). e5 expects
// asymmetric prefixes: search text uses EmbedQuery ("query: "), stored notes use
// EmbedDoc ("passage: ").
package embed

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
	ort "github.com/yalue/onnxruntime_go"
)

// Dim is the embedding dimension of multilingual-e5-small.
const Dim = 384

// maxTokens caps the sequence length; e5/XLM-R position embeddings stop at 512.
const maxTokens = 512

// inputNames/outputName match the ONNX model's IO signature (validated in the
// embedspike: int64 input_ids/attention_mask/token_type_ids -> float32
// last_hidden_state [batch, seq, 384]).
var inputNames = []string{"input_ids", "attention_mask", "token_type_ids"}

const outputName = "last_hidden_state"

// initOnce guards the process-global onnxruntime environment.
var (
	initOnce sync.Once
	initErr  error
)

// Embedder holds a loaded model + tokenizer. Safe for concurrent use: the
// onnxruntime session is not thread-safe, so Run is serialized by mu.
type Embedder struct {
	mu      sync.Mutex
	session *ort.DynamicAdvancedSession
	tk      *tokenizer.Tokenizer
}

// Open prepares the embedder, downloading whatever is missing on first use into
// modelsDir: the onnxruntime shared library (or LibEnvVar / a copy next to the
// executable, if present) and the model + tokenizer. Then it initializes
// onnxruntime and loads the session.
func Open(ctx context.Context, modelsDir string) (*Embedder, error) {
	libPath, err := ensureLib(ctx, modelsDir)
	if err != nil {
		return nil, fmt.Errorf("embed: ensure onnxruntime lib: %w", err)
	}
	modelPath, tokenizerPath, err := ensureFiles(ctx, modelsDir)
	if err != nil {
		return nil, fmt.Errorf("embed: ensure model files: %w", err)
	}

	initOnce.Do(func() {
		ort.SetSharedLibraryPath(libPath)
		initErr = ort.InitializeEnvironment()
	})
	if initErr != nil {
		return nil, fmt.Errorf("embed: init onnxruntime (lib=%q): %w", libPath, initErr)
	}

	tk, err := pretrained.FromFile(tokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("embed: load tokenizer: %w", err)
	}

	session, err := ort.NewDynamicAdvancedSession(modelPath, inputNames, []string{outputName}, nil)
	if err != nil {
		return nil, fmt.Errorf("embed: create session: %w", err)
	}

	return &Embedder{session: session, tk: tk}, nil
}

// Dim reports the embedding dimension.
func (e *Embedder) Dim() int { return Dim }

// Close releases the onnxruntime session.
func (e *Embedder) Close() error {
	if e.session != nil {
		return e.session.Destroy()
	}
	return nil
}

// EmbedQuery embeds search text (e5 "query: " prefix).
func (e *Embedder) EmbedQuery(text string) ([]float32, error) {
	return e.embed("query: " + text)
}

// EmbedDoc embeds a note to be stored/indexed (e5 "passage: " prefix).
func (e *Embedder) EmbedDoc(text string) ([]float32, error) {
	return e.embed("passage: " + text)
}

func (e *Embedder) embed(text string) ([]float32, error) {
	en, err := e.tk.EncodeSingle(text, true)
	if err != nil {
		return nil, fmt.Errorf("embed: tokenize: %w", err)
	}
	ids := truncate(en.GetIds())
	mask := truncate(en.GetAttentionMask())
	types := truncate(en.GetTypeIds())
	seq := int64(len(ids))
	if seq == 0 {
		return make([]float32, Dim), nil
	}
	shape := ort.NewShape(1, seq)

	idsT, err := ort.NewTensor(shape, toI64(ids))
	if err != nil {
		return nil, err
	}
	defer idsT.Destroy()
	maskT, err := ort.NewTensor(shape, toI64(mask))
	if err != nil {
		return nil, err
	}
	defer maskT.Destroy()
	typesT, err := ort.NewTensor(shape, toI64(types))
	if err != nil {
		return nil, err
	}
	defer typesT.Destroy()

	out, err := ort.NewEmptyTensor[float32](ort.NewShape(1, seq, Dim))
	if err != nil {
		return nil, err
	}
	defer out.Destroy()

	e.mu.Lock()
	err = e.session.Run([]ort.Value{idsT, maskT, typesT}, []ort.Value{out})
	e.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("embed: run: %w", err)
	}

	vec := meanPool(out.GetData(), mask, Dim)
	l2normalize(vec)
	return vec, nil
}

// truncate caps a token slice to maxTokens.
func truncate[T any](xs []T) []T {
	if len(xs) > maxTokens {
		return xs[:maxTokens]
	}
	return xs
}

func toI64(xs []int) []int64 {
	out := make([]int64, len(xs))
	for i, x := range xs {
		out[i] = int64(x)
	}
	return out
}

// meanPool averages token vectors over the attention mask. data is the flattened
// [seq, h] last_hidden_state; mask has length seq.
func meanPool(data []float32, mask []int, h int) []float32 {
	out := make([]float32, h)
	var n float32
	for t := 0; t < len(mask); t++ {
		if mask[t] == 0 {
			continue
		}
		n++
		base := t * h
		for j := 0; j < h; j++ {
			out[j] += data[base+j]
		}
	}
	if n > 0 {
		for j := range out {
			out[j] /= n
		}
	}
	return out
}

// l2normalize scales v to unit length in place (so dot product == cosine).
func l2normalize(v []float32) {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	if s == 0 {
		return
	}
	inv := float32(1 / math.Sqrt(s))
	for i := range v {
		v[i] *= inv
	}
}

// Cosine returns the cosine similarity of two equal-length vectors. When both
// are L2-normalized (as produced here) this is just their dot product.
func Cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
