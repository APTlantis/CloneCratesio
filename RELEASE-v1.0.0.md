# Mirror-Crates v1.0.0

**Release Date:** 2026-02-01

We are excited to announce the first stable release of Mirror-Crates, a high-performance tool for mirroring Rust crates from crates.io to a local filesystem.

## Highlights

- **Production-Ready** - Successfully mirrored ~1.8M crates (~350 GB) with a 99.995% success rate
- **High Performance** - Massive concurrency with HTTP/2 connection reuse and aggressive parallelism
- **Full Observability** - Prometheus metrics and pprof endpoints for monitoring under load
- **Resumable** - JSONL manifests enable safe restart after interruption
- **Airgap Support** - Complete documentation for offline deployment scenarios

## Features

### Download-Crates CLI

The core downloader provides:

- HTTP/2 connection pooling with configurable concurrency (default: `max(64, NumCPU * 32)`)
- SHA-256 checksum verification using crates.io-index metadata
- Exponential backoff retry with jitter for transient failures
- Sharded directory layout matching crates.io structure
- Optional rolling `tar.zst` bundles to reduce inode overhead
- JSONL manifest for audit trail and resume capability

### Generate-Sidecars CLI

Per-crate metadata generator:

- Produces JSON sidecar files alongside each `.crate` file
- Extracts version info, checksums, and yanked status from the index
- Concurrency-safe limit counter for deterministic output
- Same sharding scheme as the downloader for consistent layout

### Clone-Index.py Wrapper

Python orchestration script:

- Clones or updates the official `crates.io-index` repository
- Builds the Go downloader if not present
- Provides sensible defaults under `~/Rust-Crates/`
- Non-interactive mode for automation

### Prometheus Metrics

Six metrics exposed at `/metrics`:

| Metric | Type | Description |
|--------|------|-------------|
| `crates_download_requests_total` | Counter | Download attempts by status and HTTP code |
| `crates_download_bytes_total` | Counter | Total bytes downloaded |
| `crates_download_duration_seconds` | Histogram | Time per download attempt |
| `crates_download_retries_total` | Counter | Total retry attempts |
| `crates_download_inflight` | Gauge | Currently in-flight HTTP requests |
| `crates_processed_total` | Counter | Processed records by result |

Additional endpoints:
- `/api/status` - JSON status summary
- `/debug/pprof/*` - Go profiling endpoints

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/APTlantis/Mirror-Crates.git
cd Mirror-Crates

# Build all CLIs
go build ./cmd/...

# Or build with output directory
go build -o bin/download-crates.exe ./cmd/download-crates
go build -o bin/generate-sidecars.exe ./cmd/generate-sidecars
```

### Requirements

- Go 1.25 or newer
- Python 3.9 or newer (for the wrapper script)
- Git (for cloning crates.io-index)

## Quick Start

```bash
# Using the Python wrapper (recommended for first run)
python Clone-Index.py --output-dir ~/Rust-Crates/crates.io --threads 256

# Or use the Go CLI directly
./bin/download-crates -index-dir ~/crates.io-index -out ~/mirror -concurrency 256 -listen :9090
```

## Documentation

- [README](README.md) - Quick orientation and build instructions
- [Quickstart (Windows)](Docs/Quickstart-Windows.md) - Step-by-step Windows guide
- [Architecture](Docs/Architecture.md) - System design and data flow
- [Prometheus Guide](Docs/Prometheus.md) - Metrics reference and Grafana examples
- [Airgap Guide](Docs/Airgap-Guide.md) - Offline deployment procedures
- [Technical Overview](Docs/Technical-Overview-2025-08-23.md) - Deep-dive into implementation

## Known Limitations

- No built-in rate limiting for the source server (relies on retry backoff)
- GUI not yet available (planned for future release)
- Windows-specific: disable NTFS compression on destination for best performance

## Upgrade Notes

This is the initial stable release. No upgrade path required.

## What's Next

See the [Roadmap](README.md#roadmap-highlights) for planned features:

- GUI front-end for monitoring and verification
- Grafana dashboard templates
- Delta sync tooling for periodic updates
- Disk usage estimators for planning

## Acknowledgments

Thank you to the Rust community for building the ecosystem that makes tools like this worthwhile.

## License

MIT License. See [LICENSE](LICENSE) for details.

---

**Full Changelog:** https://github.com/APTlantis/Mirror-Crates/commits/v1.0.0

**Download:** https://github.com/APTlantis/Mirror-Crates/releases/tag/v1.0.0
