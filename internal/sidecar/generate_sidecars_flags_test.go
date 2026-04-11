package sidecar

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeIndexFile(t *testing.T, dir string, lines []string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	data := ""
	for _, l := range lines {
		data += l + "\n"
	}
	if err := os.WriteFile(dir, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestProcessIndexFile_IncludeYankedAndLimit(t *testing.T) {
	tmp := t.TempDir()
	idx := filepath.Join(tmp, "index", "s", "se", "serde")
	writeIndexFile(t, idx, []string{
		`{"name":"serde","vers":"1.0.0","cksum":"ab","yanked":false}`,
		`{"name":"serde","vers":"1.0.1","cksum":"cd","yanked":true}`,
	})

	out := filepath.Join(tmp, "out")
	cfg := Config{
		IndexDir:      filepath.Join(tmp, "index"),
		OutDir:        out,
		BaseURL:       "https://static.crates.io/crates",
		OutputMode:    OutputModeFiles,
		ManifestPath:  "",
		IncludeYanked: false,
	}

	// includeYanked=false -> only first
	limit := NewLimitCounter(10)
	ctrs := &counters{}
	if err := ProcessIndexFile(cfg, idx, limit, map[string]ManifestHint{}, ctrs, nil); err != nil && !errors.Is(err, ErrLimitReached) {
		t.Fatalf("ProcessIndexFile err: %v", err)
	}
	// Expect 1 sidecar
	dir := CrateDirFor("serde", out)
	if _, err := os.Stat(filepath.Join(dir, "serde-1.0.0.crate.json")); err != nil {
		t.Fatalf("expected sidecar for 1.0.0: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "serde-1.0.1.crate.json")); err == nil {
		t.Fatalf("did not expect sidecar for yanked 1.0.1")
	}

	// includeYanked=true with limit=1 -> only one file written
	limit2 := NewLimitCounter(1)
	ctrs2 := &counters{}
	cfg.IncludeYanked = true
	if err := ProcessIndexFile(cfg, idx, limit2, map[string]ManifestHint{}, ctrs2, nil); err != nil && !errors.Is(err, ErrLimitReached) {
		t.Fatalf("ProcessIndexFile err: %v", err)
	}
	// We should still only have two possible files, but ensure limit decremented to 0
	if limit2.Remaining() != 0 {
		t.Fatalf("expected limit2==0, got %d", limit2.Remaining())
	}
}

func TestGenerateJSONLWithManifestHints(t *testing.T) {
	tmp := t.TempDir()
	idx := filepath.Join(tmp, "index", "s", "se", "serde")
	writeIndexFile(t, idx, []string{
		`{"name":"serde","vers":"1.0.0","cksum":"ab","yanked":false}`,
	})

	manifestPath := filepath.Join(tmp, "manifest.jsonl")
	manifestLine := `{"url":"https://static.crates.io/crates/serde/serde-1.0.0.crate","storage":"bundle","bundle_path":"bundles\\bundle-0000.tar.zst","bundle_member":"static.crates.io\\serde-1.0.0.crate"}`
	if err := os.WriteFile(manifestPath, []byte(manifestLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jsonlOut := filepath.Join(tmp, "sidecars.jsonl")
	cfg := Config{
		IndexDir:      filepath.Join(tmp, "index"),
		BaseURL:       "https://static.crates.io/crates",
		OutputMode:    OutputModeJSONL,
		JSONLOut:      jsonlOut,
		ManifestPath:  manifestPath,
		Concurrency:   1,
		IncludeYanked: false,
	}

	stats, err := Generate(t.Context(), cfg)
	if err != nil {
		t.Fatalf("Generate err: %v", err)
	}
	if stats.Wrote != 1 {
		t.Fatalf("expected 1 record written, got %d", stats.Wrote)
	}

	data, err := os.ReadFile(jsonlOut)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &got); err != nil {
		t.Fatalf("unmarshal jsonl output: %v", err)
	}
	if got["storage"] != "bundle" {
		t.Fatalf("expected bundle storage, got %#v", got["storage"])
	}
	if got["bundle_path"] != "bundles\\bundle-0000.tar.zst" {
		t.Fatalf("expected bundle_path, got %#v", got["bundle_path"])
	}
	if got["bundle_member"] != "static.crates.io\\serde-1.0.0.crate" {
		t.Fatalf("expected bundle_member, got %#v", got["bundle_member"])
	}
}
