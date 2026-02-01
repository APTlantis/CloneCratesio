# Mirror-Crates

### High-Performance Rust Crates.io Mirror and Metadata Sidecar Generator

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-%3E%3D1.25-00ADD8?logo=go">
  <img alt="Python" src="https://img.shields.io/badge/Python-3.9%2B-3776AB?logo=python">
  <img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/APTlantis/Mirror-Rust-Crates">
  <a href="https://github.com/APTlantis/Mirror-Crates/releases">
    <img alt="Release" src="https://img.shields.io/github/v/release/APTlantis/Mirror-Crates?include_prereleases">
  </a>
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/License-MIT-yellow.svg"></a>
  <img alt="PRs Welcome" src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg">
  <img alt="Status" src="https://img.shields.io/badge/status-active-success">
</p>

<p align="center">
  <strong>Mirror millions of Rust crates with high throughput, full integrity verification, and operational visibility.</strong>
</p>

---

![Running the full pipeline](Docs/RunningThePipeline-Screenshot%202026-01-14%20175852.png)

**Quick links:** [Quickstart (Windows)](Docs/Quickstart-Windows.md) | [Airgap Guide](Docs/Airgap-Guide.md) | [Architecture](Docs/Architecture.md) | [Prometheus Metrics](Docs/Prometheus.md)

## Table of Contents

- [Why It Is Fast](#why-it-is-fast)
- [Features](#features)
- [Repository Layout](#repository-layout)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Build](#build)
  - [Wrapper Script](#wrapper-script)
  - [Downloader Usage](#downloader-usage)
  - [Prometheus and pprof](#prometheus-and-pprof)
  - [Sidecar Metadata Generator](#sidecar-metadata-generator)
- [Screenshots](#screenshots)
- [Development](#development)
- [Windows and WSL Notes](#windows-and-wsl-notes)
- [Roadmap Highlights](#roadmap-highlights)
- [Contributing](#contributing)
- [License](#license)

## Why It Is Fast

Traditional mirroring scripts struggle with millions of tiny `.crate` files. This project focuses on:

- **Massive concurrency** with HTTP/2 connection reuse
- **Incremental resume** via checksum-aware download verification
- **Optional bundling** into rolling `tar.zst` archives to reduce inode churn
- **Structured JSONL manifests** for auditing and restart safety
- **Prometheus metrics and pprof endpoints** for visibility under load

These design choices prioritize correctness and completion under real-world load, not synthetic benchmarks.

> "The pipeline completed a full crates.io mirror (~1.8M artifacts, ~350 GB) in ~6.5 hours with a 99.995% success rate, including verification and metadata generation."

## Features

- **Download Crates** - High-throughput HTTP/2 downloader with exponential backoff retries and SHA-256 verification
- **Generate Sidecars** - Per-crate JSON metadata files for offline tooling and registry compatibility
- **Rolling Archives** - Optional `tar.zst` bundles to reduce filesystem overhead on large mirrors
- **Prometheus Metrics** - Real-time observability with 6 metrics covering requests, bytes, duration, retries, and inflight counts
- **pprof Endpoints** - CPU and memory profiling for performance tuning
- **Resumable Downloads** - JSONL manifest tracks progress for safe restart after interruption
- **Airgap Ready** - Full documentation for offline deployment and verification

## Repository Layout

```
Clone-Index.py               Python wrapper: fetch crates.io-index and invoke Go CLIs
cmd/
  download-crates/           CLI: high-performance crate downloader
  generate-sidecars/         CLI: generate per-crate metadata sidecars
internal/
  downloader/                Download, retry, sharding, and optional bundling engine
  sidecar/                   Sidecar generation library reused by the CLI
Docs/                        Architecture, guides, and screenshots
Testdata/                    Synthetic fixtures used in unit tests
```

## Documentation Map

This repository includes multiple forms of documentation, each serving a distinct purpose:

- **README.md**  
  Quick orientation, build/run instructions, and repository layout.

- **Architecture.md**  
  System-level design, component responsibilities, and data flow.

- **Technical Overview**  
  Deep-dive into performance, concurrency, integrity, and failure modes.

- **Engineering Integrity / Architectural Philosophy**  
  Explains the values behind the design: supply-chain safety, determinism,
  and systems-level thinking.

- **Technical Q&A and Implementation Guide**  
  Practical explanations of design decisions, edge cases, and real-world usage.

- **Quickstart (Windows)**  
  Step-by-step guide for running a full mirror on Windows.

- **Airgap Guide**  
  Procedures for packaging, transporting, and verifying mirrors offline.

Screenshots and real run artifacts are included to demonstrate operational
correctness under load.

## Getting Started

### Prerequisites
- Go 1.25 or newer
- Python 3.9 or newer
- Git (for cloning the crates.io index)

See also: Docs/Quickstart-Windows.md and Docs/Airgap-Guide.md.

### Build

Build all CLIs (recommended):

```sh
go build ./cmd/...
```

Or build individually:

```powershell
go build -o bin\download-crates.exe .\cmd\download-crates
go build -o bin\generate-sidecars.exe .\cmd\generate-sidecars
```

Run without building:

```sh
go run ./cmd/download-crates -index-dir path/to/crates.io-index -out mirror-output
```

#### Wrapper Script

The Python wrapper defaults to user profile friendly paths:

```bash
python Clone-Index.py --index-dir "S:\\Rust-Crates\\crates.io-index" --output-dir "S:\\Rust-Crates\\crates.io" --threads 256 --non-interactive
```

It will clone or update the official `crates.io-index`, build or locate the downloader, and launch it with the provided thread count. All paths can be overridden via flags. Logging goes to `crate-download.log` inside the same root by default.

### Downloader Usage

```powershell
# With metrics on :9090
.\bin\download-crates.exe -index-dir "S:\Rust-Crates\crates.io-index" -out "S:\Rust-Crates\crates.io" -concurrency 256 -include-yanked -progress-interval 5s -listen :9090
```

Common options:
- `-limit` - Download only the first N entries for testing.
- `-bundle` / `-bundles-out` - Stream completed crates into rolling `tar.zst` archives.
- `-checksums` - Provide an external checksum JSONL file to enforce integrity.
- `-retries`, `-retry-base`, `-retry-max` - Configure retry policy.
- `-log-format`, `-log-level` - Structured logging (text or JSON).

### Prometheus and pprof

Expose metrics and runtime profiling by supplying `-listen :PORT`:
- Metrics: `http://localhost:PORT/metrics`
- pprof: `http://localhost:PORT/debug/pprof/`

### Sidecar Metadata Generator

```powershell
# If you built into .\bin as above
.\bin\generate-sidecars.exe -index-dir "S:\Rust-Crates\crates.io-index" -out "S:\Rust-Crates\crates.io" -concurrency 256 -include-yanked -progress-interval 5s -log-format text -log-level info
```

Sidecars (`crate-name-version.crate.json`) are written alongside the crate files using the same sharding scheme. A concurrency-safe global limit ensures predictable output when using `-limit`.

![Sidecar generator in action](Docs/SidecarsInAction-Screenshot%202026-01-14%20174545.png)

## Screenshots

<details>
<summary>Click to expand screenshots</summary>

### Project Structure

![Project structure overview](Docs/ProjectStructure-Screenshot%202026-01-14%20175353.png)

### Starting the Clone Process

![Clone-Index starting](Docs/CloneCrates-Screenshot%202026-01-14%20161843.png)

### Sidecar Generator Output

![Sidecar generator](Docs/SideCarGenerator-Screenshot%202026-01-14%20174049.png)

### Generating Sidecars (No Errors)

![Generating sidecars successfully](Docs/GeneratingSidecars-NoErrors-Screenshot%202026-01-14%20175852.png)

### Bundles and Manifest Output

![Bundles and manifest](Docs/BundlesAndManifest-Screenshot%202026-01-28%20080216.png)

</details>

## Development

- Format: `gofmt -w .` (exclude vendored toolchain: `git ls-files -z | rg -z -v "^\.tools/" | %{ $_ } | ForEach-Object { $_ }`)
- Tests: `go test ./...` (unit tests live under `internal/`)
- Lint suggestions: `go vet ./...`, `staticcheck ./...`, `golangci-lint run ./...`

### Windows and WSL Notes

- PowerShell examples use `bin\*.exe`; on WSL/Linux use `bin/*`.
- The repo includes a local Go toolchain under `.tools/go` for reproducible builds. If preferred, use your system Go 1.25+.
- Large runs benefit from fast disks (NVMe) and NTFS compression disabled on the destination directory.

## Roadmap Highlights

- [ ] GUI front-end for monitoring progress, pre-flight checks, and bundle verification
- [ ] Grafana dashboard templates for Prometheus metrics visualization
- [ ] Sample manifests and disk usage estimators for planning offline mirrors
- [ ] Delta sync tooling for periodic updates

## Contributing

Contributions are welcome. Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

For bug reports and feature requests, please [open an issue](https://github.com/APTlantis/Mirror-Crates/issues).

## License

MIT License. See [LICENSE](LICENSE).

---

<p align="center">
  Made with care for the Rust community.
</p>
