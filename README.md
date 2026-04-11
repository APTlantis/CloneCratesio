---
[project]
name = "Cratesio"
slug = "cratesio"

description = "High-Performance Rust Crates.io Mirror and Metadata Sidecar Generator"

[tags]
languages = ["go", "python", "rust"]
platforms = ["windows", "linux"]
tooling = ["git"]
---

# Mirror-Crates

### High-Performance Rust Crates.io Mirror, Bundle, and Sidecar Pipeline

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-%3E%3D1.25-00ADD8?logo=go">
  <img alt="Python" src="https://img.shields.io/badge/Python-3.9%2B-3776AB?logo=python">
  <img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/APTlantis/CloneCratesio">
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/License-MIT-yellow.svg"></a>
  <img alt="Status" src="https://img.shields.io/badge/status-active-success">
</p>

<p align="center">
  <strong>Mirror crates.io as loose files or low-inode rolling bundles, then generate sidecars that match the storage mode you chose.</strong>
</p>

---

![Running the full pipeline](Docs/RunningThePipeline-Screenshot%202026-01-14%20175852.png)

**Quick links:** [Quickstart (Windows)](Docs/Quickstart-Windows.md) | [Airgap Guide](Docs/Airgap-Guide.md) | [Architecture](Docs/Architecture.md) | [Prometheus Metrics](Docs/Prometheus.md)

## Why It Is Fast

Traditional mirroring scripts struggle with millions of tiny `.crate` files. This project focuses on:

- **High concurrency** with HTTP/2 connection reuse
- **Fast incremental updates** by trusting existing crate files during routine syncs
- **Explicit re-verification** with `-verify-existing` when you want a full checksum pass
- **Rolling bundles** for low-inode workflows
- **Structured JSONL audit logs** for run history, bundle metadata, and sidecar export
- **Prometheus metrics and pprof** for live operational visibility

## What It Can Do

- **Download Crates** as a normal crates.io-style shard tree
- **Bundle Crates** into rolling `tar.zst` archives with per-bundle manifests and a top-level bundle index
- **Generate Loose Sidecars** as `name-version.crate.json` files next to loose crates
- **Generate JSONL Sidecars** for bundled workflows without creating millions of tiny metadata files
- **Extract Bundles** back into the normal crates.io shard layout
- **Expose Metrics** automatically on `:9090` by default

## Repository Layout

```text
Clone-Index.py               Python wrapper: clone/update index and launch downloader
cmd/
  download-crates/           CLI: mirror crates as loose files or bundles
  extract-bundles/           CLI: restore bundle archives into shard layout
  generate-sidecars/         CLI: write sidecars as files or JSONL
internal/
  downloader/                Download, retry, bundle, manifest, and metrics engine
  sidecar/                   Sidecar generation library
Docs/                        Architecture, guides, and screenshots
Testdata/                    Synthetic fixtures used in unit tests
```

## Build

Build all CLIs:

```sh
go build ./cmd/...
```

Or build individual binaries:

```powershell
go build -o bin\download-crates.exe .\cmd\download-crates
go build -o bin\generate-sidecars.exe .\cmd\generate-sidecars
go build -o bin\extract-bundles.exe .\cmd\extract-bundles
```

## Recommended Workflows

### 1. First Full Mirror as Loose Files

Use this when you want a normal shard tree with `.crate` files on disk.

```powershell
python Clone-Index.py --index-dir "S:\Rust-Crates\crates.io-index" --output-dir "S:\Rust-Crates\crates.io" --threads 128 --non-interactive
```

Or run the downloader directly:

```powershell
.\bin\download-crates.exe -index-dir "S:\Rust-Crates\crates.io-index" -out "S:\Rust-Crates\crates.io" -concurrency 128 -include-yanked -progress-interval 5s
```

Then generate loose sidecars:

```powershell
.\bin\generate-sidecars.exe -index-dir "S:\Rust-Crates\crates.io-index" -out "S:\Rust-Crates\crates.io" -output-mode files -concurrency 128 -include-yanked
```

### 2. Weekly Update Run

The downloader now trusts existing crate files by default, so routine update runs do not re-hash the whole mirror.

```powershell
.\bin\download-crates.exe -index-dir "S:\Rust-Crates\crates.io-index" -out "S:\Rust-Crates\crates.io" -concurrency 128 -progress-interval 5s
```

If you want a full existing-file integrity sweep:

```powershell
.\bin\download-crates.exe -index-dir "S:\Rust-Crates\crates.io-index" -out "S:\Rust-Crates\crates.io" -verify-existing -concurrency 128
```

### 3. Low-Inode Bundle Workflow

Use this when the main goal is reducing inode pressure and avoiding a second full loose-file copy.

```powershell
.\bin\download-crates.exe -index-dir "S:\Rust-Crates\crates.io-index" -out "S:\Rust-Crates\staging" -bundle -bundle-mode only -bundle-size-gb 8 -bundles-out "S:\Rust-Crates\bundles" -manifest "S:\Rust-Crates\manifest.jsonl" -concurrency 128
```

Important notes:

- `-bundle-mode only` is the default and removes loose `.crate` files after they are written into the bundle
- `-bundle-mode add` keeps both the loose files and the bundle
- Each bundle now gets its own manifest JSONL
- The bundles directory also gets a top-level `bundles.index.jsonl`

Generate JSONL sidecars for the bundled mirror:

```powershell
.\bin\generate-sidecars.exe -index-dir "S:\Rust-Crates\crates.io-index" -output-mode jsonl -jsonl-out "S:\Rust-Crates\bundle-sidecars.jsonl" -manifest "S:\Rust-Crates\manifest.jsonl" -concurrency 128 -include-yanked
```

### 4. Restore a Bundle Set to Loose Files

Bundle archives now preserve the normal shard layout inside the tar, so extraction recreates the expected directory tree directly.

```powershell
.\bin\extract-bundles.exe -bundles-dir "S:\Rust-Crates\bundles" -out "S:\Rust-Crates\restored"
```

Use `-overwrite` if you want existing files replaced.

## Downloader Notes

Common options:

- `-concurrency` - Number of concurrent downloads. Default: `128`
- `-verify-existing` - Re-hash and verify existing crate files instead of trusting them during update runs
- `-bundle` - Enable rolling `tar.zst` bundling
- `-bundle-mode only|add` - Choose whether bundled runs remove or keep loose crate files
- `-bundle-size-gb` - Target bundle size in GB. Default: `8`
- `-bundles-out` - Directory for `.tar.zst` bundles, per-bundle manifests, and `bundles.index.jsonl`
- `-manifest` - Write the JSONL audit log for the run
- `-listen :PORT` - Override the default metrics listener (`:9090`); pass an empty string to disable it
- startup logs now print the effective run configuration before work begins
- completion logs now print a cleaner end-of-run summary with processed, downloaded, existing, verified, elapsed, and rate

## Sidecar Notes

Sidecars now intentionally follow the storage mode:

- `-output-mode files` is for loose-file mirrors and writes `name-version.crate.json`
- `-output-mode jsonl` is for bundle workflows and writes one aggregated JSONL stream
- `-manifest` can enrich JSONL sidecars with `storage`, `bundle_path`, and `bundle_member`

## Prometheus and pprof

The downloader starts metrics and runtime profiling on `:9090` by default. Override it with `-listen :PORT` if you want a different address, or pass an empty string to disable it:

- Metrics: `http://localhost:PORT/metrics`
- JSON status: `http://localhost:PORT/api/status`
- pprof: `http://localhost:PORT/debug/pprof/`

See [Docs/Prometheus.md](Docs/Prometheus.md) for metric details.

## Development

- Format: `gofmt -w .`
- Tests: `go test ./...`
- Lint suggestions: `go vet ./...`, `staticcheck ./...`, `golangci-lint run ./...`

## Windows and WSL Notes

- PowerShell examples use `bin\*.exe`; on WSL/Linux use `bin/*`
- The repo includes a local Go toolchain under `.tools/go`
- Large runs benefit from fast disks and predictable destination paths

## Roadmap Highlights

- [ ] GUI front-end for monitoring progress, pre-flight checks, and bundle verification
- [ ] Grafana dashboard templates for Prometheus metrics visualization
- [ ] Sample manifest and bundle-inspection utilities
- [ ] Delta sync tooling for periodic updates

## License

MIT License. See [LICENSE](LICENSE).
The Python wrapper now also forwards the day-to-day downloader flags that matter most, including `--include-yanked`, `--verify-existing`, `--bundle`, `--bundle-mode`, `--bundle-size-gb`, `--bundles-out`, `--manifest`, `--listen`, `--progress-interval`, and `--dry-run`.
