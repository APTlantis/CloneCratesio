# Mirror Rust crates from crates.io

## Clone Index
```
options:
  -h, --help            show this help message and exit
  --index-dir INDEX_DIR
                        Path to local crates.io index (default: C:\Users\Administrator\Rust-Crates\crates.io-index)
  --output-dir OUTPUT_DIR
                        Directory where .crate files will be saved (default: C:\Users\Administrator\Rust-Crates\crates)
  --log-path LOG_PATH   Path to log file for this wrapper (default: C:\Users\Administrator\Rust-Crates\crate-download.log)
  --threads THREADS     Number of download threads (mapped to -concurrency for Download-Crates)
  --rate-limit RATE_LIMIT
                        Deprecated: no direct equivalent in Download-Crates; kept for compatibility
  --resume              Deprecated: no direct equivalent; kept for compatibility
  --verify              Deprecated: verification handled by Download-Crates; kept for compatibility
  --skip-index-update   Skip updating the crates.io index
  --non-interactive, --yes
                        Do not prompt; proceed automatically (CI-friendly)
  --log-level {debug,info,warning,error}
                        Logging level for this wrapper (default: info)
  --downloader-path DOWNLOADER_PATH
                        Path to Download-Crates binary; if empty, auto-detect or fallback to 'go run'
```

### Command
```
python Clone-Index.py --index-dir "A:\rust-lang\crates\crates.io-index" --output-dir "A:\rust-lang\crates\crates.io"
```

## Generate Sidecars
```
PS D:\Crates\cmd\generate-sidecars> go run main.go --help
Usage of C:\Users\ADMINI~1\AppData\Local\Temp\go-build731731142\b001\exe\main.exe:
  -concurrency int
        Number of concurrent index-file workers (default 128)
  -crates-base-url string
        Base URL for crates content (default "https://static.crates.io/crates")
  -include-yanked
        Include yanked versions from the index
  -index-dir string
        Path to local crates.io-index directory (e.g., C:\Rust-Crates\crates.io-index)
  -limit int
        Limit number of entries to write (0 = all)
  -log-format string
        Logging format: text|json (default "text")
  -log-level string
        Logging level: debug|info|warn|error (default "info")
  -out string
        Directory to write sidecar metadata files (default "out")
  -progress-every int
        Log progress every N processed items (0=disabled)
  -progress-interval duration
        Periodic progress logging interval (e.g., 5s; 0=disabled)
```
### Command
```
./main --index-dir "A:\rust-lang\crates\crates.io-index" --out "A:\rust-lang\crates\crates.io" --include-yanked
```

## Downloading
```
Usage of C:\Users\ADMINI~1\AppData\Local\Temp\go-build3655089033\b001\exe\main.exe:
  -bundle
        Enable rolling tar.zst bundling while downloading
  -bundle-size-gb int
        Target bundle size in GB (default 8)
  -bundles-out string
        Directory for .tar.zst bundles (default "bundles")
  -checksums string
        Optional JSONL of {url, sha256}
  -concurrency int
        Number of concurrent downloads (default 512)
  -crates-base-url string
        Base URL for crates content (default "https://static.crates.io/crates")
  -dry-run
        Validate inputs and estimate work; do not download
  -idle-timeout duration
        Override http.Transport IdleConnTimeout (0=auto)
  -include-yanked
        Include yanked versions from the index
  -index-dir string
        Path to local crates.io-index directory (e.g., C:\Rust-Crates\crates.io-index)
  -limit int
        Limit number of crates to process (0 = no limit)
  -list string
        Path to newline-delimited URL list
  -listen string
        Serve Prometheus metrics and pprof at this address (e.g., :9090)
  -log-format string
        Logging format: text|json (default "text")
  -log-level string
        Logging level: debug|info|warn|error (default "info")
  -manifest string
        Where to write records (JSONL) (default "manifest.jsonl")
  -max-conns-per-host int
        Override http.Transport MaxConnsPerHost (0=auto)
  -max-idle-conns int
        Override http.Transport MaxIdleConns (0=auto)
  -max-idle-per-host int
        Override http.Transport MaxIdleConnsPerHost (0=auto)
  -out string
        Directory to store downloaded files (default "out")
  -progress-every int
        Log progress every N processed items (0=disabled)
  -progress-interval duration
        Periodic progress logging interval (e.g., 5s; 0=disabled)
  -retries int
        Total retry attempts for transient errors (default 6)
  -retry-base duration
        Base backoff for retries (exponential with jitter) (default 500ms)
  -retry-max duration
        Max backoff per attempt (default 30s)
  -timeout int
        Per-request timeout in seconds (default 300)
  -tls-timeout duration
        Override http.Transport TLSHandshakeTimeout (0=auto)
```

### Command
```
./main --index-dir "A:\rust-lang\crates\crates.io-index" --out "A:\rust-lang\crates\crates.io" --include-yanked --listen :9595 --bundle --bundle-size-gb 5 --bundles-out "A:\rust-lang\3-13-26" PS A:\AptWeb\zypper-operations\Cratesio\cmd\download-crates> go run main.go --help
```