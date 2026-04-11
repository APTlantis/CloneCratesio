# Mirror-Rust-Crates — Action Plan

Last updated: 2025-09-30

## Executive Summary
- Core tooling now has structured logging, resilient retries, Prometheus/pprof endpoints, fast incremental update behavior, bundle-first workflows, and dual sidecar modes.
- Repository hygiene improved (module paths fixed, binaries removed, sane defaults in Clone-Index.py, synchronized defaults across CLIs).
- Next milestones focus on documentation polish, release packaging, and UI/inspection tooling for bundle-heavy workflows.

## Current Architecture Snapshot
1. **Clone-Index.py** - Clones/updates `crates.io-index`, provisions default directories under the user profile, and invokes the Go downloader.
2. **Download-Crates** - High-throughput HTTP/2 downloader with JSONL audit logging, fast incremental update mode, optional re-verification, optional rolling bundles, and metrics.
3. **Generate-Sidecars** - Emits either per-version `.crate.json` files or aggregated JSONL sidecars, depending on the storage workflow.
4. **Extract-Bundles** - Restores rolling `tar.zst` bundles into the standard crates.io shard layout.

## Recent Improvements
- Removed committed binaries and refreshed `.gitignore` to keep `go.sum` while excluding build artefacts.
- Added safe retry accounting and metrics handling in the downloader to avoid negative gauges and misreported retries.
- Introduced a thread-safe limit counter for sidecar generation, eliminating races detected by `-race` and ensuring deterministic output.
- Fixed file descriptor leaks in bundler TAR packaging and added bundle catalogs plus per-bundle manifests.
- Default Python wrapper paths now use `%USERPROFILE%/Rust-Crates`, create log directories automatically, and run `git pull` via `cwd` without global `chdir`.
- Standardized downloader and sidecar defaults to `128`.
- Incremental update runs now trust existing files by default, with opt-in `-verify-existing`.
- Bundle mode now defaults to `only`, avoiding silent duplicate storage.
- Bundle archives now preserve shard paths internally and can be extracted with `extract-bundles`.
- Sidecars now support both loose-file and JSONL bundle-oriented output.
- README and architecture docs rewritten around the current workflows.

## Open Issues & Opportunities
### Repository Structure
- `go test ./...` now passes. The next step is improving integration coverage around end-to-end bundle and extraction workflows.
- Continue growing shared package coverage instead of falling back to CLI-only testing.

### Observability & UX
- Bundle Prometheus dashboards and example Grafana panels so operators can monitor throughput quickly.
- Publish sample manifests, bundle catalogs, and inspection utilities so operators can diff index vs. mirror state quickly.

### GUI Initiative
- Prototype a cross-platform desktop UI (Go + Fyne or web front-end) that: 
  1. Validates prerequisites (disk space, index freshness).
  2. Launches clone/download/sidecar jobs with progress bars fed from Prometheus endpoints.
  3. Visualises bandwidth, retries, and bundle creation.
  4. Provides post-run verification workflows (hash inventory, signature reporting).

### Documentation
- Add diagrams (PNG/SVG committed) and a manifest schema appendix to the architecture doc.
- Expand the current workflow docs with dedicated HOWTO pages: "Mirror in 10 Minutes", "Periodic Delta Sync", "Bundle Workflow", and "Air-Gapped Restore".
- Add CONTRIBUTING, CODEOWNERS, and issue/PR templates aimed at outside contributors.

### Release Engineering
- Add a `cmd` build matrix to GitHub Actions and publish release assets (`Download-Crates`, `Generate-Sidecars`).
- Introduce `make`/`just` targets for `build`, `test`, `lint`, `mirror-sample` to simplify onboarding.

## Next Five Actions
1. Finish the remaining docs/help cleanup so bundle workflows, JSONL sidecars, and extraction are discoverable without reading code.
2. Create GitHub Actions workflow(s) for the current layout, including race detector and lint jobs.
3. Add integration-style tests for bundle creation, bundle extraction, and bundle-sidecar enrichment.
4. Draft CONTRIBUTING.md and CODE_OF_CONDUCT.md to encourage community participation.
5. Define GUI MVP requirements and technical stack decision, documenting it in `Docs/gui-roadmap.md`.

## Risk Notes
- Large refactors may introduce regressions in download performance; protect with benchmarks or integration tests on a small sample index.
- Moving binaries into `cmd/` requires updating existing automation scripts; coordinate documentation and wrapper changes simultaneously.
- GUI work may require additional dependencies; plan for vendoring/offline builds.
