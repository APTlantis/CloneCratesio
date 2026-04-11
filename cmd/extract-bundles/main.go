package main

import (
	"archive/tar"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"
)

func main() {
	var (
		bundlesDir = flag.String("bundles-dir", "", "Directory containing .tar.zst bundles")
		pattern    = flag.String("pattern", "*.tar.zst", "Glob pattern to match within -bundles-dir")
		outDir     = flag.String("out", "", "Directory where crate files will be extracted")
		overwrite  = flag.Bool("overwrite", false, "Overwrite files that already exist at the destination")
	)
	flag.Parse()

	if *bundlesDir == "" || *outDir == "" {
		flag.CommandLine.SetOutput(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage: extract-bundles -bundles-dir <path> -out <dir> [options]")
		flag.PrintDefaults()
		os.Exit(2)
	}

	matches, err := filepath.Glob(filepath.Join(*bundlesDir, *pattern))
	if err != nil {
		slog.Error("invalid glob pattern", "err", err)
		os.Exit(2)
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		slog.Error("no bundles matched", "bundles_dir", *bundlesDir, "pattern", *pattern)
		os.Exit(1)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		slog.Error("failed to create output directory", "err", err)
		os.Exit(1)
	}

	var extracted int64
	for _, bundlePath := range matches {
		n, err := extractBundle(bundlePath, *outDir, *overwrite)
		if err != nil {
			slog.Error("bundle extraction failed", "bundle", bundlePath, "err", err)
			os.Exit(1)
		}
		extracted += n
		slog.Info("bundle_extracted", "bundle", bundlePath, "files", n)
	}

	slog.Info("extraction_done", "bundles", len(matches), "files", extracted, "out", *outDir)
}

func extractBundle(bundlePath, outDir string, overwrite bool) (int64, error) {
	f, err := os.Open(bundlePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	zr, err := zstd.NewReader(f)
	if err != nil {
		return 0, err
	}
	defer zr.Close()

	tr := tar.NewReader(zr)
	var count int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return count, nil
		}
		if err != nil {
			return count, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		destPath, err := safeJoin(outDir, hdr.Name)
		if err != nil {
			return count, err
		}
		if !overwrite {
			if _, err := os.Stat(destPath); err == nil {
				continue
			}
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return count, err
		}
		if err := writeFileFromTar(destPath, tr, hdr.FileInfo().Mode()); err != nil {
			return count, err
		}
		count++
	}
}

func safeJoin(root, member string) (string, error) {
	cleanMember := filepath.Clean(filepath.FromSlash(member))
	if cleanMember == "." || strings.HasPrefix(cleanMember, "..") || filepath.IsAbs(cleanMember) {
		return "", fmt.Errorf("unsafe bundle member path %q", member)
	}
	return filepath.Join(root, cleanMember), nil
}

func writeFileFromTar(destPath string, r io.Reader, mode os.FileMode) error {
	tmpPath := destPath + ".part"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if mode != 0 {
		if err := os.Chmod(tmpPath, mode.Perm()); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
