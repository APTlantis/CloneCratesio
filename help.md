# Mirror Rust crates from crates.io

## Clone Index

```text
python Clone-Index.py --help

options:
  --index-dir INDEX_DIR
  --output-dir OUTPUT_DIR
  --log-path LOG_PATH
  --threads THREADS                      default: 128
  --include-yanked
  --verify-existing
  --bundle
  --bundle-mode {only,add}               default: only
  --bundle-size-gb BUNDLE_SIZE_GB        default: 8
  --bundles-out BUNDLES_OUT
  --manifest MANIFEST
  --listen LISTEN                        default: :9090
  --progress-interval PROGRESS_INTERVAL  default: 5s
  --progress-every PROGRESS_EVERY
  --dry-run
  --skip-index-update
  --non-interactive, --yes
  --log-level {debug,info,warning,error}
  --downloader-path DOWNLOADER_PATH

legacy compatibility flags:
  --rate-limit
  --resume
  --verify
```

Example:

```powershell
python Clone-Index.py --index-dir "A:\rust-lang\crates\crates.io-index" --output-dir "A:\rust-lang\crates\crates.io" --threads 128 --progress-interval 5s --non-interactive
```

## Download Crates

```text
go run ./cmd/download-crates -h

important flags:
  -index-dir string
  -out string
  -concurrency int                     default: 128
  -verify-existing
  -bundle
  -bundle-mode string                  only|add, default: only
  -bundle-size-gb int                  default: 8
  -bundles-out string
  -manifest string                     JSONL audit log for the run
  -include-yanked
  -listen string                       default :9090; use :PORT to override or empty string to disable
  -progress-interval duration
  -log-format string                   text|json
  -log-level string                    debug|info|warn|error
```

Loose-file mirror:

```powershell
.\bin\download-crates.exe -index-dir "A:\rust-lang\crates\crates.io-index" -out "A:\rust-lang\crates\crates.io" -concurrency 128 -include-yanked
```

Fast update run:

```powershell
.\bin\download-crates.exe -index-dir "A:\rust-lang\crates\crates.io-index" -out "A:\rust-lang\crates\crates.io" -concurrency 128
```

Full re-verification of existing files:

```powershell
.\bin\download-crates.exe -index-dir "A:\rust-lang\crates\crates.io-index" -out "A:\rust-lang\crates\crates.io" -verify-existing -concurrency 128
```

Bundle-first mirror:

```powershell
.\bin\download-crates.exe -index-dir "A:\rust-lang\crates\crates.io-index" -out "A:\rust-lang\crates\staging" -bundle -bundle-mode only -bundle-size-gb 5 -bundles-out "A:\rust-lang\bundles" -manifest "A:\rust-lang\manifest.jsonl" -concurrency 128
```

Notes:

- `-bundle-mode only` removes loose `.crate` files after they are added to the archive
- `-bundle-mode add` keeps both loose files and bundles
- bundled runs now produce per-bundle manifests and a top-level `bundles.index.jsonl`
- the downloader prints a cleaner startup configuration block and a final run summary

## Generate Sidecars

```text
go run ./cmd/generate-sidecars -h

important flags:
  -index-dir string
  -out string
  -output-mode string                  files|jsonl, default: files
  -jsonl-out string                    required for jsonl mode
  -manifest string                     optional downloader manifest for bundle metadata
  -concurrency int                     default: 128
  -include-yanked
  -progress-interval duration
  -log-format string                   text|json
  -log-level string                    debug|info|warn|error
```

Loose sidecars:

```powershell
.\bin\generate-sidecars.exe -index-dir "A:\rust-lang\crates\crates.io-index" -out "A:\rust-lang\crates\crates.io" -output-mode files -include-yanked -concurrency 128
```

Bundle-oriented JSONL sidecars:

```powershell
.\bin\generate-sidecars.exe -index-dir "A:\rust-lang\crates\crates.io-index" -output-mode jsonl -jsonl-out "A:\rust-lang\bundle-sidecars.jsonl" -manifest "A:\rust-lang\manifest.jsonl" -include-yanked -concurrency 128
```

## Extract Bundles

```text
go run ./cmd/extract-bundles -h

important flags:
  -bundles-dir string
  -pattern string                      default: *.tar.zst
  -out string
  -overwrite
```

Example:

```powershell
.\bin\extract-bundles.exe -bundles-dir "A:\rust-lang\bundles" -out "A:\rust-lang\restored"
```

## Metrics

The downloader exposes metrics on `:9090` by default. Use `-listen :PORT` to override it, or pass an empty string to disable it.

Endpoints:

- `http://localhost:PORT/metrics`
- `http://localhost:PORT/api/status`
- `http://localhost:PORT/debug/pprof/`
