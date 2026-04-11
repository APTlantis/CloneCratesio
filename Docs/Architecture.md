# Mirror-Crates Architecture

This document describes the current crates.io mirroring pipeline as it actually works today: loose-file mirrors, rolling bundle mirrors, sidecar generation, extraction, and runtime observability.

---

## Overview

The pipeline starts from a local `crates.io-index` checkout and expands it into one of two storage models:

- **Loose-file mirror**
  - Stores each `.crate` file in a crates.io-style shard directory
  - Best when you want direct file access and per-crate adjacency

- **Bundle mirror**
  - Stores crates in rolling `tar.zst` archives
  - Best when you want lower inode usage and more compact transport units
  - Bundle mode defaults to `only`, so it does not silently keep a second loose-file copy

Sidecars follow the storage model:

- Loose-file mirrors use per-crate `.crate.json` files
- Bundle mirrors use aggregated JSONL sidecar output

---

## Components

### Clone-Index.py

- Clones or updates `crates.io-index`
- Uses user-profile-friendly defaults
- Forwards the common downloader workflow flags so the wrapper and Go CLI behave consistently
- Prints a wrapper-level start summary and a final elapsed-time summary
- Invokes the downloader with the selected thread count

### Download-Crates

- Reads a local `crates.io-index` or a URL list
- Builds crate download URLs and checksum hints
- Downloads with HTTP/2 and retry logic
- Writes a JSONL audit log for the run
- Optionally writes rolling bundles
- Exposes Prometheus metrics and pprof on `:9090` by default

Important flags:

- `-index-dir`
- `-out`
- `-concurrency`
- `-verify-existing`
- `-bundle`
- `-bundle-mode only|add`
- `-bundle-size-gb`
- `-bundles-out`
- `-manifest`
- `-listen`

### Generate-Sidecars

- Reads index files and emits metadata derived from the original index lines
- Supports two output modes:
  - `files` for loose-file mirrors
  - `jsonl` for bundled mirrors
- Can enrich sidecars from the downloader manifest to include bundle storage metadata

Important flags:

- `-index-dir`
- `-out`
- `-output-mode files|jsonl`
- `-jsonl-out`
- `-manifest`
- `-include-yanked`

### Extract-Bundles

- Reads one or more `.tar.zst` bundle files
- Restores their contents into the normal crates.io shard layout
- Preserves the archive member paths exactly as stored in the bundle

Important flags:

- `-bundles-dir`
- `-pattern`
- `-out`
- `-overwrite`

---

## Data Flow

### 1. Clone or Update the Index

`Clone-Index.py` keeps a local `crates.io-index` checkout current.

### 2. Expand the Index into Crate URLs

`Download-Crates` scans the local index and produces URLs like:

`https://static.crates.io/crates/{name}/{name}-{version}.crate`

If the index entry contains `cksum`, that checksum is used for verification.

### 3. Decide How Existing Files Are Treated

The downloader has two update behaviors:

- **Default**
  - Existing files are trusted and counted as `existing`
  - This makes routine update runs fast

- **With `-verify-existing`**
  - Existing files are re-hashed and counted as `verified_existing`
  - If verification fails, the file is re-downloaded

### 4. Write Loose Files or Bundles

When bundling is disabled:

- Crates are written to the shard tree under `-out`

When bundling is enabled:

- Crates are still downloaded through the normal file path
- Each completed crate is streamed into the current rolling `tar.zst`
- The bundle stores the crate under the same shard path used by the loose-file mirror
- Storage behavior then depends on `-bundle-mode`

Bundle modes:

- `only`
  - Default
  - Removes the loose `.crate` after it has been added to the bundle

- `add`
  - Keeps both the loose `.crate` and the bundle copy

### 5. Emit Bundle Metadata

Each bundled run now produces:

- the archive itself, such as `bundle-0003.tar.zst`
- a per-bundle manifest, such as `bundle-0003-serde-to-zstd.manifest.jsonl`
- a top-level bundle catalog: `bundles.index.jsonl`

The per-bundle manifest contains one record per crate member.

The top-level catalog contains one record per completed bundle and is meant for fast discovery:

- `bundle_path`
- `manifest_path`
- `crate_min`
- `crate_max`
- `entries`
- `size_bytes`

### 6. Generate Sidecars

Loose-file workflow:

- `generate-sidecars -output-mode files`
- writes `name-version.crate.json` next to loose crates

Bundle workflow:

- `generate-sidecars -output-mode jsonl`
- writes one large JSONL metadata stream instead of millions of tiny files
- optional `-manifest` input enriches each record with:
  - `storage`
  - `bundle_path`
  - `bundle_member`

### 7. Extract Bundles Back to a Normal Tree

`extract-bundles` reads archive members and writes them back under the shard path stored in the tar. No extra path reconstruction logic is needed because the archive member name already preserves the desired layout.

---

## File Layouts

### Loose-File Mirror Layout

Mirror artifact layout is derived from the crate name using two shard directories.

Examples:

- `serde` -> `s/er/serde-<vers>.crate`
- `ab` -> `ab/ab-<vers>.crate`
- `1serde` -> `1/se/1serde-<vers>.crate`

### Bundle Member Layout

Bundles now store members using the same shard-relative path used by the loose-file mirror.

Example:

- archive member `s/er/serde-1.0.0.crate`

That means extraction recreates the normal shard tree directly.

### Sidecar Layout

- `files` mode:
  - `s/er/serde-1.0.0.crate.json`

- `jsonl` mode:
  - one JSONL file chosen by `-jsonl-out`

---

## Manifest Schema

### Downloader Audit Log

The downloader manifest is a JSONL audit log of what happened during the run.

Key fields include:

- `schema_version`
- `url`
- `path`
- `storage`
- `bundle_path`
- `bundle_member`
- `size`
- `sha256`
- `started_at`
- `finished_at`
- `ok`
- `status`
- `retries`
- `error`

Common statuses now include:

- `existing`
- `verified_existing`
- `downloaded`
- `error`

### Bundle Catalog

The bundle catalog is `bundles.index.jsonl` and contains one record per completed bundle.

### Sidecar JSONL

Bundle-oriented sidecars are emitted as one JSONL stream, with each line containing the original index-derived metadata plus any bundle hints imported from the downloader manifest.

---

## Observability

By default the downloader exposes these endpoints on `:9090`. Use `-listen :PORT` to override the address, or pass an empty string to disable the listener:

- `/metrics`
- `/api/status`
- `/debug/pprof/`

The JSON status endpoint includes:

- `processed`
- `ok`
- `errors`
- `downloaded`
- `existing`
- `verified_existing`
- `uptime_sec`
- `rate_per_sec`

See [Prometheus.md](Prometheus.md) for metric details.

---

## Tests

Current automated coverage includes:

- shard path helpers
- checksum verification
- incremental update behavior
- bundle mode behavior
- bundle catalog generation
- sidecar file mode
- sidecar JSONL mode with manifest enrichment

`go test ./...` currently passes.

---

## Operational Guidance

- Use loose-file mode when direct per-crate access matters more than inode count
- Use bundle mode when inode pressure and transportability matter more than per-file browsing
- Use JSONL sidecars with bundle workflows
- Use `-verify-existing` only when you want a deliberate integrity sweep
