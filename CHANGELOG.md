# Changelog

All notable changes to this project will be documented in this file.

The format is inspired by Keep a Changelog. Dates are YYYY-MM-DD.

## [1.1.0] - 2026-04-11

### Changed
- Standardized downloader, sidecar, and wrapper defaults to `128` concurrency
- Made Prometheus and pprof start automatically on `:9090` by default
- Improved wrapper startup/final summaries and downloader run summaries
- Reframed the run manifest as a JSONL audit log

### Added
- `-verify-existing` mode for full checksum sweeps of existing files
- Explicit bundle modes: `only|add`
- Per-bundle manifest JSONL files
- Top-level `bundles.index.jsonl`
- `extract-bundles` CLI for restoring bundle archives into shard layout
- JSONL sidecar mode for bundled workflows

### Fixed
- Weekly update runs no longer behave like near-full re-verification passes by default
- Bundle-first workflows no longer silently duplicate loose crate storage
- Bundle archives now preserve shard layout internally
- Documentation now matches the current pipeline and metrics behavior


