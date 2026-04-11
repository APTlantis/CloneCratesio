# Project TODO — derived from Action-Plan.md

Generated: 2025-09-30
Source: Action-Plan.md (Last updated: 2025-09-30)

Legend: [ ] not started | [x] done | [~] in progress

---

## Structural Refactor
- [x] Move CLI entry points into `cmd/` directories and extract shared logic into reusable packages.
- [x] Update tests to exercise new packages and ensure `go test ./...` passes.
- [ ] Add integration test harness using synthetic index fixtures.

## CI and Tooling
- [ ] Update GitHub Actions workflows for new layout (build, test, lint, race).
- [ ] Publish release artifacts (binaries + checksums) on tagged builds.
- [ ] Add `Makefile` or `justfile` targets for build/test/lint/mirror-sample.

## Documentation
- [x] Rewrite README with ASCII-only content and current instructions.
- [x] Expand `Docs/Architecture.md` with current pipeline behavior, bundle semantics, and manifest notes.
- [x] Add HOWTO guidance for loose mirrors, fast update runs, bundle workflows, JSONL sidecars, and extraction.
- [ ] Create CONTRIBUTING.md, CODEOWNERS, and issue/PR templates.
- [ ] Document GUI roadmap in `Docs/gui-roadmap.md`.

## Observability & UX
- [ ] Provide Grafana dashboard examples for metrics exposed by the downloader.
- [ ] Supply sample manifests and diff tooling documentation.

## GUI Initiative
- [ ] Decide on GUI stack and MVP scope.
- [ ] Prototype GUI wiring to downloader metrics (progress, retries, throughput).

## Release Readiness
- [x] Remove committed binaries and clean `.gitignore`.
- [x] Fix module paths for root module.
- [x] Ensure Clone-Index defaults are portable and non-interruptive.
- [x] Harden downloader retries and metrics behaviour.
- [x] Make sidecar limits concurrency-safe.
- [x] Standardize downloader and sidecar defaults to `128`.
- [x] Implement fast incremental update mode with opt-in `-verify-existing`.
- [x] Add bundle modes (`only|add`), per-bundle manifests, and bundle extraction tooling.
- [x] Add JSONL sidecar mode for bundled workflows.

---

Notes
- `go test ./...` now passes; keep extending coverage around bundle workflows and extraction.
- Keep documentation ASCII-only to avoid rendering artefacts in terminals and GitHub UI.
- When refactoring packages, update the Python wrapper to call the relocated binaries or shared library.
