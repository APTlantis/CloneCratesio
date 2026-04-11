package sidecar

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultConcurrency = 128

const (
	OutputModeFiles = "files"
	OutputModeJSONL = "jsonl"
)

type Config struct {
	IndexDir         string
	OutDir           string
	IncludeYanked    bool
	Limit            int64
	Concurrency      int
	BaseURL          string
	ProgressInterval time.Duration
	ProgressEvery    int
	OutputMode       string
	JSONLOut         string
	ManifestPath     string
}

type Stats struct {
	FilesScanned int64
	Wrote        int64
	Skipped      int64
	Errors       int64
	Duration     time.Duration
}

type counters struct {
	mu      sync.Mutex
	total   int64
	wrote   int64
	skipped int64
	errors  int64
}

func (c *counters) addTotal(n int64) { c.mu.Lock(); c.total += n; c.mu.Unlock() }
func (c *counters) incWrote()        { c.mu.Lock(); c.wrote++; c.mu.Unlock() }
func (c *counters) incSkipped()      { c.mu.Lock(); c.skipped++; c.mu.Unlock() }
func (c *counters) incErrors()       { c.mu.Lock(); c.errors++; c.mu.Unlock() }
func (c *counters) snapshot() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{FilesScanned: c.total, Wrote: c.wrote, Skipped: c.skipped, Errors: c.errors}
}

type LimitCounter struct {
	mu        sync.Mutex
	remaining int64
}

func NewLimitCounter(total int64) *LimitCounter {
	return &LimitCounter{remaining: total}
}

func (lc *LimitCounter) Reserve() bool {
	if lc == nil {
		return true
	}
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if lc.remaining <= 0 {
		return false
	}
	lc.remaining--
	return true
}

func (lc *LimitCounter) Release() {
	if lc == nil {
		return
	}
	lc.mu.Lock()
	lc.remaining++
	lc.mu.Unlock()
}

func (lc *LimitCounter) Remaining() int64 {
	if lc == nil {
		return 0
	}
	lc.mu.Lock()
	defer lc.mu.Unlock()
	return lc.remaining
}

type ManifestHint struct {
	Storage      string `json:"storage"`
	BundlePath   string `json:"bundle_path"`
	BundleMember string `json:"bundle_member"`
}

type JSONLWriter struct {
	mu sync.Mutex
	f  *os.File
}

func NewJSONLWriter(path string) (*JSONLWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &JSONLWriter{f: f}, nil
}

func (w *JSONLWriter) WriteRecord(record map[string]any) error {
	b, err := json.Marshal(record)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.f.Write(b); err != nil {
		return err
	}
	_, err = w.f.Write([]byte("\n"))
	return err
}

func (w *JSONLWriter) Close() error {
	if w == nil || w.f == nil {
		return nil
	}
	return w.f.Close()
}

var ErrLimitReached = errors.New("sidecar limit reached")

func DefaultConcurrency() int {
	return defaultConcurrency
}

func Generate(ctx context.Context, cfg Config) (Stats, error) {
	if cfg.IndexDir == "" {
		return Stats{}, errors.New("index dir is required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://static.crates.io/crates"
	}
	if cfg.OutputMode == "" {
		cfg.OutputMode = OutputModeFiles
	}
	if cfg.OutputMode != OutputModeFiles && cfg.OutputMode != OutputModeJSONL {
		return Stats{}, fmt.Errorf("invalid output mode %q", cfg.OutputMode)
	}
	if cfg.OutputMode == OutputModeFiles && cfg.OutDir == "" {
		return Stats{}, errors.New("out dir is required for files mode")
	}
	if cfg.OutputMode == OutputModeJSONL && cfg.JSONLOut == "" {
		return Stats{}, errors.New("jsonl output path is required for jsonl mode")
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency()
	}
	if concurrency > 1024 {
		concurrency = 1024
	}

	var files []string
	if err := filepath.Walk(cfg.IndexDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == ".github" || name == ".gitignore" {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		bn := info.Name()
		if bn == "config.json" || strings.EqualFold(bn, "README.md") || strings.HasSuffix(bn, ".keep") {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return Stats{}, err
	}

	if len(files) == 0 {
		return Stats{}, fmt.Errorf("no index files found under %s", cfg.IndexDir)
	}

	if cfg.OutputMode == OutputModeFiles {
		if err := os.MkdirAll(cfg.OutDir, 0o755); err != nil {
			return Stats{}, err
		}
	}

	manifestHints, err := ReadManifestHints(cfg.ManifestPath)
	if err != nil {
		return Stats{}, err
	}

	var jsonlWriter *JSONLWriter
	if cfg.OutputMode == OutputModeJSONL {
		jsonlWriter, err = NewJSONLWriter(cfg.JSONLOut)
		if err != nil {
			return Stats{}, err
		}
		defer jsonlWriter.Close()
	}

	jobs := make(chan string, sidecarMax(1024, concurrency*2))
	var wg sync.WaitGroup
	ctrs := &counters{}
	var limitBudget *LimitCounter
	if cfg.Limit > 0 {
		limitBudget = NewLimitCounter(cfg.Limit)
	}

	errCh := make(chan error, concurrency)

	worker := func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case path, ok := <-jobs:
				if !ok {
					return
				}
				if limitBudget != nil && limitBudget.Remaining() <= 0 {
					continue
				}
				if err := ProcessIndexFile(cfg, path, limitBudget, manifestHints, ctrs, jsonlWriter); err != nil {
					if errors.Is(err, ErrLimitReached) {
						return
					}
					ctrs.incErrors()
					select {
					case errCh <- err:
					default:
					}
				}
			}
		}
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker()
	}

	start := time.Now()
	if cfg.ProgressInterval > 0 || cfg.ProgressEvery > 0 {
		interval := cfg.ProgressInterval
		if interval <= 0 {
			interval = 250 * time.Millisecond
		}
		ticker := time.NewTicker(interval)
		go func() {
			defer ticker.Stop()
			var lastReported int64
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					snap := ctrs.snapshot()
					processed := snap.Wrote + snap.Skipped + snap.Errors
					if cfg.ProgressEvery > 0 && processed-lastReported < int64(cfg.ProgressEvery) {
						continue
					}
					elapsed := time.Since(start)
					var rate float64
					if elapsed > 0 {
						rate = float64(processed) / elapsed.Seconds()
					}
					slog.Info("sidecar_progress", "processed", processed, "wrote", snap.Wrote, "skipped", snap.Skipped, "errors", snap.Errors, "files_scanned", snap.FilesScanned, "elapsed", elapsed.String(), "rate_per_sec", fmt.Sprintf("%.1f", rate))
					lastReported = processed
				}
			}
		}()
	}

	startOut := cfg.OutDir
	if cfg.OutputMode == OutputModeJSONL {
		startOut = cfg.JSONLOut
	}
	slog.Info("sidecar_start", "files", len(files), "concurrency", concurrency, "mode", cfg.OutputMode, "out", startOut)

loop:
	for _, f := range files {
		if limitBudget != nil && limitBudget.Remaining() <= 0 {
			break
		}
		select {
		case <-ctx.Done():
			break loop
		case jobs <- f:
		}
	}
	close(jobs)
	wg.Wait()

	select {
	case err := <-errCh:
		if err != nil {
			return Stats{}, err
		}
	default:
	}

	stats := ctrs.snapshot()
	stats.Duration = time.Since(start)
	slog.Info("sidecar_done", "wrote", stats.Wrote, "skipped", stats.Skipped, "errors", stats.Errors, "files_scanned", stats.FilesScanned, "elapsed", stats.Duration.String())
	return stats, nil
}

func ProcessIndexFile(cfg Config, indexPath string, limit *LimitCounter, manifestHints map[string]ManifestHint, ctrs *counters, jsonlWriter *JSONLWriter) error {
	f, err := os.Open(indexPath)
	if err != nil {
		return err
	}
	defer f.Close()

	relIndex := indexPath
	if rel, err := filepath.Rel(cfg.IndexDir, indexPath); err == nil {
		relIndex = filepath.ToSlash(rel)
	}

	s := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024*1024)
	s.Buffer(buf, 64*1024*1024)

	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ctrs.addTotal(1)

		if limit != nil && limit.Remaining() <= 0 {
			return ErrLimitReached
		}

		record, name, vers, err := buildSidecarRecord(line, relIndex, cfg.BaseURL, cfg.IncludeYanked, manifestHints)
		if err != nil {
			if errors.Is(err, errSkipRecord) {
				ctrs.incSkipped()
				continue
			}
			ctrs.incErrors()
			continue
		}

		limitReserved := false
		if limit != nil {
			if !limit.Reserve() {
				return ErrLimitReached
			}
			limitReserved = true
		}

		switch cfg.OutputMode {
		case OutputModeFiles:
			if err := writeSidecarFile(cfg.OutDir, name, vers, record); err != nil {
				if limitReserved {
					limit.Release()
				}
				if errors.Is(err, errSkipRecord) {
					ctrs.incSkipped()
					continue
				}
				ctrs.incErrors()
				continue
			}
		case OutputModeJSONL:
			if err := jsonlWriter.WriteRecord(record); err != nil {
				if limitReserved {
					limit.Release()
				}
				ctrs.incErrors()
				continue
			}
		}

		ctrs.incWrote()
	}
	if err := s.Err(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

var errSkipRecord = errors.New("skip record")

func buildSidecarRecord(line, relIndex, baseURL string, includeYanked bool, manifestHints map[string]ManifestHint) (map[string]any, string, string, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		return nil, "", "", err
	}
	name, _ := m["name"].(string)
	vers, _ := m["vers"].(string)
	if name == "" || vers == "" {
		return nil, "", "", errSkipRecord
	}
	if !includeYanked {
		if y, ok := m["yanked"].(bool); ok && y {
			return nil, "", "", errSkipRecord
		}
	}

	crateFile := fmt.Sprintf("%s-%s.crate", name, vers)
	crateURL := fmt.Sprintf("%s/%s/%s", strings.TrimRight(baseURL, "/"), name, crateFile)
	m["crate_file"] = crateFile
	m["crate_url"] = crateURL
	m["index_path"] = relIndex

	if hint, ok := manifestHints[crateURL]; ok {
		if hint.Storage != "" {
			m["storage"] = hint.Storage
		}
		if hint.BundlePath != "" {
			m["bundle_path"] = hint.BundlePath
		}
		if hint.BundleMember != "" {
			m["bundle_member"] = hint.BundleMember
		}
	}

	return m, name, vers, nil
}

func writeSidecarFile(outDir, name, vers string, record map[string]any) error {
	dir := CrateDirFor(name, outDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	sidecarName := fmt.Sprintf("%s-%s.crate.json", name, vers)
	outPath := filepath.Join(dir, sidecarName)

	if _, err := os.Stat(outPath); err == nil {
		return errSkipRecord
	}

	tmpPath := outPath + ".tmp"
	of, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(of)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(record); err != nil {
		of.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := of.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, outPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func ReadManifestHints(path string) (map[string]ManifestHint, error) {
	if path == "" {
		return map[string]ManifestHint{}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	type manifestRecord struct {
		URL          string `json:"url"`
		Storage      string `json:"storage"`
		BundlePath   string `json:"bundle_path"`
		BundleMember string `json:"bundle_member"`
	}

	out := make(map[string]ManifestHint)
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		var rec manifestRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec.URL == "" {
			continue
		}
		out[rec.URL] = ManifestHint{
			Storage:      rec.Storage,
			BundlePath:   rec.BundlePath,
			BundleMember: rec.BundleMember,
		}
	}
	return out, s.Err()
}

// CrateDirFor mirrors the shard layout used for crate artifacts.
func CrateDirFor(crateName string, outDir string) string {
	if crateName == "" {
		return outDir
	}
	name := crateName
	if len(name) <= 3 {
		return filepath.Join(outDir, name)
	}
	var firstDir string
	if strings.HasPrefix(name, "1") || strings.HasPrefix(name, "2") || strings.HasPrefix(name, "3") {
		firstDir = name[:1]
	} else {
		if len(name) >= 2 && len(name) > 1 && name[1] == '-' {
			firstDir = name[:2]
		} else {
			firstDir = name[:1]
		}
	}
	secondStart := len(firstDir)
	secondEnd := secondStart + 2
	if secondEnd > len(name) {
		secondEnd = len(name)
	}
	secondDir := name[secondStart:secondEnd]
	return filepath.Join(outDir, firstDir, secondDir)
}

func sidecarMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
