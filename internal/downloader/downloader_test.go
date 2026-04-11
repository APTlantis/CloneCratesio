package downloader

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCrateDirFor(t *testing.T) {
	out := t.TempDir()
	// Short names (<=3)
	if got := crateDirFor("ab", out); got != filepath.Join(out, "ab") {
		t.Fatalf("crateDirFor short: got %q", got)
	}
	if got := crateDirFor("abc", out); got != filepath.Join(out, "abc") {
		t.Fatalf("crateDirFor 3-len: got %q", got)
	}
	// Normal name
	if got := crateDirFor("serde", out); got != filepath.Join(out, "s", "er") {
		t.Fatalf("crateDirFor serde: got %q", got)
	}
	// Starts with digit 1..3 -> first dir is first char
	if got := crateDirFor("1serde", out); got != filepath.Join(out, "1", "se") {
		t.Fatalf("crateDirFor 1serde: got %q", got)
	}
}

func TestSanitizeName(t *testing.T) {
	u := "https://static.crates.io/crates/serde/serde-1.0.0.crate"
	if got := sanitizeName(u); got != "serde-1.0.0.crate" {
		t.Fatalf("sanitizeName: got %q", got)
	}
	u2 := "https://example.com/x/file?foo=1&bar=2"
	got := sanitizeName(u2)
	if !strings.Contains(got, "_") {
		t.Fatalf("sanitizeName should replace special chars: %q", got)
	}
}

func TestBundleMemberPath(t *testing.T) {
	out := t.TempDir()
	path := filepath.Join(out, "s", "er", "serde-1.0.0.crate")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	got := bundleMemberPath(out, path)
	if got != "s/er/serde-1.0.0.crate" {
		t.Fatalf("bundleMemberPath unexpected: %q", got)
	}
}

func TestVerifyFile(t *testing.T) {
	d := &Downloader{checksums: map[string]string{}}
	f := filepath.Join(t.TempDir(), "x.bin")
	content := []byte("hello world\n")
	if err := os.WriteFile(f, content, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	sum := sha256.Sum256(content)
	url := "https://example.com/x.bin"
	d.checksums[url] = hex.EncodeToString(sum[:])
	ok, got := d.verifyFile(f, url)
	if !ok {
		t.Fatalf("verifyFile should pass, got sum=%s", got)
	}
	d.checksums[url] = strings.Repeat("0", 64)
	ok, _ = d.verifyFile(f, url)
	if ok {
		t.Fatalf("verifyFile should fail with wrong checksum")
	}
}

func TestFetchOneTrustsExistingFileByDefault(t *testing.T) {
	out := t.TempDir()
	url := "https://static.crates.io/crates/serde/serde-1.0.0.crate"
	path := filepath.Join(crateDirFor("serde", out), "serde-1.0.0.crate")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	var manifest bytes.Buffer
	d := NewDownloader(out, 1, time.Second, map[string]string{url: strings.Repeat("0", 64)}, &manifest, nil, false, BundleModeOnly)
	rec := d.fetchOne(t.Context(), url, nil)
	if !rec.OK || rec.Status != "existing" {
		t.Fatalf("expected existing status, got ok=%v status=%q", rec.OK, rec.Status)
	}
	if rec.Path != path {
		t.Fatalf("expected existing path %q, got %q", path, rec.Path)
	}
}

func TestFetchOneVerifiesExistingFileWhenRequested(t *testing.T) {
	out := t.TempDir()
	url := "https://static.crates.io/crates/serde/serde-1.0.0.crate"
	content := []byte("existing")
	path := filepath.Join(crateDirFor("serde", out), "serde-1.0.0.crate")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)

	var manifest bytes.Buffer
	d := NewDownloader(out, 1, time.Second, map[string]string{url: hex.EncodeToString(sum[:])}, &manifest, nil, true, BundleModeOnly)
	rec := d.fetchOne(t.Context(), url, nil)
	if !rec.OK || rec.Status != "verified_existing" {
		t.Fatalf("expected verified_existing status, got ok=%v status=%q", rec.OK, rec.Status)
	}
	if rec.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("expected sha256 %q, got %q", hex.EncodeToString(sum[:]), rec.SHA256)
	}
}

func TestFetchOneRedownloadsWhenExistingVerificationFails(t *testing.T) {
	out := t.TempDir()
	urlPath := "/crates/serde/serde-1.0.0.crate"
	newContent := []byte("downloaded")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != urlPath {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.Copy(w, bytes.NewReader(newContent))
	}))
	defer server.Close()

	url := server.URL + urlPath
	path := filepath.Join(crateDirFor("serde", out), "serde-1.0.0.crate")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(newContent)

	var manifest bytes.Buffer
	d := NewDownloader(out, 1, time.Second, map[string]string{url: hex.EncodeToString(sum[:])}, &manifest, nil, true, BundleModeOnly)
	d.client = server.Client()

	rec := d.fetchOne(t.Context(), url, nil)
	if !rec.OK || rec.Status != "downloaded" {
		t.Fatalf("expected downloaded status, got ok=%v status=%q", rec.OK, rec.Status)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(newContent) {
		t.Fatalf("expected redownloaded content %q, got %q", string(newContent), string(got))
	}
}

func TestFetchOneBundleOnlyRemovesLooseFile(t *testing.T) {
	out := t.TempDir()
	bundlesOut := filepath.Join(out, "bundles")
	urlPath := "/crates/serde/serde-1.0.0.crate"
	content := []byte("downloaded")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != urlPath {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.Copy(w, bytes.NewReader(content))
	}))
	defer server.Close()

	sum := sha256.Sum256(content)
	bndl, err := NewBundler(true, bundlesOut, 1)
	if err != nil {
		t.Fatalf("NewBundler: %v", err)
	}
	defer bndl.Close()

	var manifest bytes.Buffer
	d := NewDownloader(out, 1, time.Second, map[string]string{server.URL + urlPath: hex.EncodeToString(sum[:])}, &manifest, bndl, false, BundleModeOnly)
	d.client = server.Client()

	rec := d.fetchOne(t.Context(), server.URL+urlPath, nil)
	if !rec.OK || rec.Status != "downloaded" {
		t.Fatalf("expected downloaded status, got ok=%v status=%q", rec.OK, rec.Status)
	}
	if rec.Storage != "bundle" {
		t.Fatalf("expected bundle storage, got %q", rec.Storage)
	}
	if rec.Path != "" {
		t.Fatalf("expected loose path to be removed, got %q", rec.Path)
	}
	if rec.BundlePath == "" || rec.BundleMember == "" {
		t.Fatalf("expected bundle metadata to be recorded: %+v", rec)
	}
	loosePath := filepath.Join(crateDirFor("serde", out), "serde-1.0.0.crate")
	if _, err := os.Stat(loosePath); !os.IsNotExist(err) {
		t.Fatalf("expected loose crate file to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(rec.BundlePath); err != nil {
		t.Fatalf("expected bundle file to exist: %v", err)
	}
}

func TestFetchOneBundleAddKeepsLooseFile(t *testing.T) {
	out := t.TempDir()
	bundlesOut := filepath.Join(out, "bundles")
	urlPath := "/crates/serde/serde-1.0.0.crate"
	content := []byte("downloaded")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != urlPath {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.Copy(w, bytes.NewReader(content))
	}))
	defer server.Close()

	sum := sha256.Sum256(content)
	bndl, err := NewBundler(true, bundlesOut, 1)
	if err != nil {
		t.Fatalf("NewBundler: %v", err)
	}
	defer bndl.Close()

	var manifest bytes.Buffer
	d := NewDownloader(out, 1, time.Second, map[string]string{server.URL + urlPath: hex.EncodeToString(sum[:])}, &manifest, bndl, false, BundleModeAdd)
	d.client = server.Client()

	rec := d.fetchOne(t.Context(), server.URL+urlPath, nil)
	if !rec.OK || rec.Status != "downloaded" {
		t.Fatalf("expected downloaded status, got ok=%v status=%q", rec.OK, rec.Status)
	}
	if rec.Storage != "filesystem+bundle" {
		t.Fatalf("expected filesystem+bundle storage, got %q", rec.Storage)
	}
	if rec.Path == "" {
		t.Fatalf("expected loose path to be retained")
	}
	if _, err := os.Stat(rec.Path); err != nil {
		t.Fatalf("expected loose crate file to exist: %v", err)
	}
	if _, err := os.Stat(rec.BundlePath); err != nil {
		t.Fatalf("expected bundle file to exist: %v", err)
	}
}

func TestBundlerRotation(t *testing.T) {
	// Create two small files
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.txt")
	b := filepath.Join(tmp, "b.txt")
	if err := os.WriteFile(a, []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte(strings.Repeat("B", 1024)), 0o644); err != nil {
		t.Fatal(err)
	}

	bundlesOut := filepath.Join(tmp, "bundles")
	// targetGB=0 rotates on every add
	bndl, err := NewBundler(true, bundlesOut, 0)
	if err != nil {
		t.Fatalf("NewBundler: %v", err)
	}
	defer bndl.Close()

	if _, err := bndl.AddFile(a, "a.txt", Record{URL: "https://example.com/a.txt"}, "a"); err != nil {
		t.Fatalf("AddFile a: %v", err)
	}
	if _, err := bndl.AddFile(b, "b.txt", Record{URL: "https://example.com/b.txt"}, "b"); err != nil {
		t.Fatalf("AddFile b: %v", err)
	}
	_ = bndl.Close()
	// Allow FS to flush on slow runners
	time.Sleep(50 * time.Millisecond)

	// Expect at least two bundle files
	entries, err := os.ReadDir(bundlesOut)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("expected >=2 bundle files, got %d", len(entries))
	}
	catalogPath := filepath.Join(bundlesOut, "bundles.index.jsonl")
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("expected bundle catalog: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	if len(lines) < 2 {
		t.Fatalf("expected >=2 catalog entries, got %d", len(lines))
	}
	var first map[string]any
	if err := json.Unmarshal(lines[0], &first); err != nil {
		t.Fatalf("unmarshal catalog entry: %v", err)
	}
	if first["bundle_path"] == "" || first["manifest_path"] == "" {
		t.Fatalf("catalog entry missing paths: %+v", first)
	}
	runtime.KeepAlive(bndl)
}

func TestReadCratesFromIndex_FlagsAndLimit(t *testing.T) {
	tmp := t.TempDir()
	// Synthesize a tiny index
	idxFile := filepath.Join(tmp, "s", "se", "serde")
	if err := os.MkdirAll(filepath.Dir(idxFile), 0o755); err != nil {
		t.Fatal(err)
	}
	data := ""
	data += `{"name":"serde","vers":"1.0.0","cksum":"` + strings.Repeat("a", 64) + `","yanked":false}` + "\n"
	data += `{"name":"serde","vers":"1.0.1","cksum":"` + strings.Repeat("b", 64) + `","yanked":true}` + "\n"
	if err := os.WriteFile(idxFile, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	// includeYanked=false
	urls, sums, err := ReadCratesFromIndex(tmp, "https://static.crates.io/crates", false, 0)
	if err != nil {
		t.Fatalf("ReadCratesFromIndex err: %v", err)
	}
	if len(urls) != 1 {
		t.Fatalf("expect 1 url, got %d", len(urls))
	}
	if len(sums) != 1 {
		t.Fatalf("expect 1 checksum, got %d", len(sums))
	}

	// includeYanked=true, limit=1
	urls2, _, err := ReadCratesFromIndex(tmp, "https://static.crates.io/crates", true, 1)
	if err != nil {
		t.Fatalf("ReadCratesFromIndex err: %v", err)
	}
	if got := len(urls2); got != 1 {
		t.Fatalf("limit not applied, got %d", got)
	}
}
