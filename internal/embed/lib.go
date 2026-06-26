package embed

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// LibEnvVar overrides the native library path; handy for `task dev` or tests.
const LibEnvVar = "GIX_ONNXRUNTIME_LIB"

// ortVersion is the onnxruntime release we download. Must provide ORT API >= 26
// (required by onnxruntime_go v1.31).
const ortVersion = "1.27.0"

// libFileName is the stable on-disk name we give the extracted shared library.
func libFileName() string {
	switch runtime.GOOS {
	case "windows":
		return "onnxruntime.dll"
	case "darwin":
		return "libonnxruntime.dylib"
	default:
		return "libonnxruntime.so"
	}
}

// libToken matches the shared library entry inside the release archive.
func libToken() string {
	switch runtime.GOOS {
	case "windows":
		return ".dll"
	case "darwin":
		return ".dylib"
	default:
		return ".so"
	}
}

// ortArchiveURL is the official onnxruntime release asset for this OS/arch, plus
// whether it's a zip (Windows) or a gzipped tar (everything else).
func ortArchiveURL() (url string, isZip bool, err error) {
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	if arch == "" {
		return "", false, fmt.Errorf("embed: unsupported arch %q", runtime.GOARCH)
	}
	base := "https://github.com/microsoft/onnxruntime/releases/download/v" + ortVersion + "/"
	switch runtime.GOOS {
	case "windows":
		return base + fmt.Sprintf("onnxruntime-win-%s-%s.zip", arch, ortVersion), true, nil
	case "darwin":
		return base + fmt.Sprintf("onnxruntime-osx-%s-%s.tgz", arch, ortVersion), false, nil
	case "linux":
		return base + fmt.Sprintf("onnxruntime-linux-%s-%s.tgz", arch, ortVersion), false, nil
	default:
		return "", false, fmt.Errorf("embed: unsupported OS %q", runtime.GOOS)
	}
}

// ensureLib returns the onnxruntime shared library path: the LibEnvVar override,
// else a copy already sitting next to the executable, else it downloads the
// official release into modelsDir and extracts the library on first use.
func ensureLib(ctx context.Context, modelsDir string) (string, error) {
	if p := os.Getenv(LibEnvVar); p != "" {
		return p, nil
	}
	if exe, err := os.Executable(); err == nil {
		if p := filepath.Join(filepath.Dir(exe), libFileName()); fileExists(p) {
			return p, nil
		}
	}

	dir := filepath.Join(modelsDir, "onnxruntime-"+ortVersion)
	libPath := filepath.Join(dir, libFileName())
	if fileExists(libPath) {
		return libPath, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	url, isZip, err := ortArchiveURL()
	if err != nil {
		return "", err
	}
	archive, err := downloadTemp(ctx, url, "onnxruntime")
	if err != nil {
		return "", err
	}
	defer os.Remove(archive)

	if isZip {
		err = extractLibFromZip(archive, libPath)
	} else {
		err = extractLibFromTarGz(archive, libPath)
	}
	if err != nil {
		return "", fmt.Errorf("embed: extract onnxruntime lib: %w", err)
	}
	return libPath, nil
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// downloadTemp streams url to a temp file and returns its path. The caller
// removes it. Progress is reported via OnDownloadProgress under `label`.
func downloadTemp(ctx context.Context, url, label string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", label, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: status %s", label, resp.Status)
	}

	tmp, err := os.CreateTemp("", "gix-ort-*.archive")
	if err != nil {
		return "", err
	}
	src := io.Reader(resp.Body)
	if OnDownloadProgress != nil {
		src = &progressReader{r: resp.Body, file: label, total: resp.ContentLength}
	}
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("download %s: %w", label, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

// isLibEntry reports whether an archive path is the shared library we want: a
// file under a lib/ dir carrying the platform token (.dll/.so/.dylib).
func isLibEntry(name string) bool {
	name = strings.ToLower(filepath.ToSlash(name))
	return strings.Contains(name, "/lib/") &&
		strings.Contains(name, "onnxruntime") &&
		strings.Contains(name, libToken())
}

func extractLibFromZip(archivePath, libPath string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !isLibEntry(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = writeFile(libPath, rc)
		rc.Close()
		return err
	}
	return fmt.Errorf("no lib entry found in %s", filepath.Base(archivePath))
}

func extractLibFromTarGz(archivePath, libPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// Real library file (skip symlinks, which tar marks as TypeSymlink).
		if hdr.Typeflag != tar.TypeReg || !isLibEntry(hdr.Name) {
			continue
		}
		return writeFile(libPath, tr)
	}
	return fmt.Errorf("no lib entry found in %s", filepath.Base(archivePath))
}

// writeFile copies r into a temp file beside dst, then renames it into place so
// an interrupted extraction never leaves a half-written library.
func writeFile(dst string, r io.Reader) error {
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".lib-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}
