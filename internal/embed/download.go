package embed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Model artifacts (multilingual-e5-small, int8 ONNX export by Xenova). Sizes are
// used as a lightweight integrity guard against truncated/partial downloads.
const (
	modelDirName = "multilingual-e5-small"

	modelFile = "model_quantized.onnx"
	modelURL  = "https://huggingface.co/Xenova/multilingual-e5-small/resolve/main/onnx/model_quantized.onnx"
	modelSize = 118308185

	tokenizerFile = "tokenizer.json"
	tokenizerURL  = "https://huggingface.co/Xenova/multilingual-e5-small/resolve/main/tokenizer.json"
	tokenizerSize = 17082730
)

// OnDownloadProgress, if set, is called periodically while model files download.
// total is -1 when the server doesn't report Content-Length.
var OnDownloadProgress func(file string, downloaded, total int64)

// ensureFiles makes sure the model and tokenizer exist under modelsDir,
// downloading any that are missing or the wrong size. Returns their paths.
func ensureFiles(ctx context.Context, modelsDir string) (modelPath, tokenizerPath string, err error) {
	dir := filepath.Join(modelsDir, modelDirName)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	modelPath = filepath.Join(dir, modelFile)
	tokenizerPath = filepath.Join(dir, tokenizerFile)

	if err = ensureFile(ctx, modelPath, modelURL, modelSize); err != nil {
		return "", "", err
	}
	if err = ensureFile(ctx, tokenizerPath, tokenizerURL, tokenizerSize); err != nil {
		return "", "", err
	}
	return modelPath, tokenizerPath, nil
}

// ensureFile downloads url to path unless a file of the expected size is already
// there. Downloads to a temp file and renames, so an interrupted download never
// leaves a corrupt file in place.
func ensureFile(ctx context.Context, path, url string, wantSize int64) error {
	if fi, err := os.Stat(path); err == nil && fi.Size() == wantSize {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", filepath.Base(path), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %s", filepath.Base(path), resp.Status)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".dl-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename

	src := io.Reader(resp.Body)
	if OnDownloadProgress != nil {
		src = &progressReader{r: resp.Body, file: filepath.Base(path), total: resp.ContentLength}
	}
	if _, err = io.Copy(tmp, src); err != nil {
		tmp.Close()
		return fmt.Errorf("download %s: %w", filepath.Base(path), err)
	}
	if err = tmp.Close(); err != nil {
		return err
	}

	if fi, statErr := os.Stat(tmpName); statErr == nil && wantSize > 0 && fi.Size() != wantSize {
		return fmt.Errorf("download %s: size %d, expected %d", filepath.Base(path), fi.Size(), wantSize)
	}
	return os.Rename(tmpName, path)
}

type progressReader struct {
	r     io.Reader
	file  string
	total int64
	done  int64
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	OnDownloadProgress(p.file, p.done, p.total)
	return n, err
}
