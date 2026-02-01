## Airgap Guide

This guide covers producing a portable crates mirror and verifying it offline.

### 1) Produce a manifest while downloading
Run the downloader to emit `manifest.jsonl`:
```sh
download-crates \
  -index-dir /data/crates.io-index \
  -out /data/crates-mirror \
  -concurrency 256 \
  -listen :9090 \
  -log-format json
```

The downloader writes `manifest.jsonl` at the repo root (or in `-out` if configured) with one JSON record per crate version, including path, size, and hash.

### 2) Package for transport (optional)
To reduce inode count and copy times, bundle into rolling archives:
```sh
download-crates \
  -index-dir /data/crates.io-index \
  -out /data/crates-mirror \
  -bundle \
  -bundles-out /data/crates-bundles
```

### 3) Generate sidecars
```sh
generate-sidecars \
  -index-dir /data/crates.io-index \
  -out /data/crates-mirror \
  -concurrency 256
```

### 4) Verify manifest integrity
The manifest contains SHA-256 hashes for all downloaded files. Use standard tools to verify:
```sh
# Extract paths and hashes from manifest
jq -r 'select(.ok==true) | "\(.sha256)  \(.path)"' manifest.jsonl > checksums.txt

# Verify with sha256sum (Linux) or Get-FileHash (Windows)
sha256sum -c checksums.txt
```

### 5) Move into the airgapped environment
Copy either the raw mirror directory or bundle archives and the manifest/artifacts. Use tools like `robocopy` (Windows) or `rsync` (Linux) to preserve timestamps and retry on transient errors.

### 6) Verify integrity offline
- Recompute SHA-256 hashes on arrival and compare against the manifest.
- Verify a random sample of crates to ensure paths, sizes, and checksums align.
- Check sidecar files match the expected index metadata.

### Notes
- Manifest Schema: stable keys (name, version, path, size, sha256, yanked, timestamp).
- Backwards compatibility: include `schema_version` in the first record and bump on changes.
- Preservation: keep `crates.io-index` commit hash captured during mirroring for provenance.

