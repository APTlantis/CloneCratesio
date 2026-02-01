# Prometheus Metrics Guide

Mirror-Crates exposes Prometheus metrics and pprof endpoints for real-time observability during large-scale mirroring operations.

## Enabling Metrics

Start the downloader with the `-listen` flag to expose metrics on a specific port:

```powershell
.\bin\download-crates.exe -index-dir "S:\Rust-Crates\crates.io-index" -out "S:\Rust-Crates\crates.io" -listen :9090
```

## Available Endpoints

| Endpoint | Description |
|----------|-------------|
| `/metrics` | Prometheus scrape endpoint (OpenMetrics format) |
| `/api/status` | JSON status summary for dashboards and scripts |
| `/debug/pprof/` | Go pprof index for CPU/memory profiling |
| `/debug/pprof/profile` | CPU profile (30s default) |
| `/debug/pprof/heap` | Heap memory profile |
| `/debug/pprof/trace` | Execution trace |

## Metrics Reference

### crates_download_requests_total

**Type:** Counter (with labels)

Tracks all download attempts, labeled by status and HTTP response code.

| Label | Values | Description |
|-------|--------|-------------|
| `status` | `ok`, `error` | Whether the request succeeded |
| `code` | HTTP status code or `net` | Response code or network error |

**Example queries:**
```promql
# Total successful downloads
crates_download_requests_total{status="ok"}

# Failed requests by HTTP code
sum by (code) (crates_download_requests_total{status="error"})

# Download success rate
rate(crates_download_requests_total{status="ok"}[5m]) / rate(crates_download_requests_total[5m])
```

### crates_download_bytes_total

**Type:** Counter

Total bytes downloaded across all successful requests.

**Example queries:**
```promql
# Current throughput (bytes/sec)
rate(crates_download_bytes_total[1m])

# Total GB downloaded
crates_download_bytes_total / 1024 / 1024 / 1024
```

### crates_download_duration_seconds

**Type:** Histogram

Time spent per download attempt. Uses default Prometheus buckets.

**Example queries:**
```promql
# Average download latency
rate(crates_download_duration_seconds_sum[5m]) / rate(crates_download_duration_seconds_count[5m])

# 95th percentile latency
histogram_quantile(0.95, rate(crates_download_duration_seconds_bucket[5m]))
```

### crates_download_retries_total

**Type:** Counter

Total number of retry attempts across all downloads.

**Example queries:**
```promql
# Retry rate per minute
rate(crates_download_retries_total[1m])

# Retries as percentage of total requests
crates_download_retries_total / crates_download_requests_total * 100
```

### crates_download_inflight

**Type:** Gauge

Number of HTTP requests currently in-flight. Useful for tuning concurrency.

**Example queries:**
```promql
# Current inflight requests
crates_download_inflight

# Average inflight over time
avg_over_time(crates_download_inflight[5m])
```

### crates_processed_total

**Type:** Counter (with labels)

Tracks processed records by result type.

| Label | Values | Description |
|-------|--------|-------------|
| `result` | `ok`, `error`, `skipped` | Processing outcome |

**Example queries:**
```promql
# Completed downloads (new files)
crates_processed_total{result="ok"}

# Skipped (already exists with valid checksum)
crates_processed_total{result="skipped"}

# Error rate
rate(crates_processed_total{result="error"}[5m])
```

## JSON Status Endpoint

The `/api/status` endpoint returns a JSON object for quick status checks:

```json
{
  "version": "dev",
  "processed": 1500000,
  "ok": 1499850,
  "errors": 150,
  "uptime_sec": 3600,
  "rate_per_sec": "416.7"
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `version` | string | Build version identifier |
| `processed` | int | Total records processed |
| `ok` | int | Successful downloads |
| `errors` | int | Failed downloads |
| `uptime_sec` | int | Seconds since process start |
| `rate_per_sec` | string | Current processing rate |

**Usage example:**
```bash
# Check status with curl
curl -s http://localhost:9090/api/status | jq

# Monitor progress in a loop
watch -n 5 'curl -s http://localhost:9090/api/status | jq'
```

## Prometheus Configuration

Add Mirror-Crates to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'mirror-crates'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 15s
```

## Grafana Dashboard

Example panel queries for a Grafana dashboard:

### Throughput Panel
```promql
rate(crates_download_bytes_total[1m]) / 1024 / 1024
```
*Unit: MiB/s*

### Progress Panel
```promql
crates_processed_total{result="ok"} + crates_processed_total{result="skipped"}
```
*Shows total completed items*

### Error Rate Panel
```promql
rate(crates_processed_total{result="error"}[5m]) * 60
```
*Unit: errors/min*

### Concurrency Utilization
```promql
crates_download_inflight
```
*Compare against your `-concurrency` setting*

## pprof Profiling

When performance tuning is needed, use the pprof endpoints:

```bash
# CPU profile (30 seconds)
go tool pprof http://localhost:9090/debug/pprof/profile

# Heap profile
go tool pprof http://localhost:9090/debug/pprof/heap

# Goroutine dump
curl http://localhost:9090/debug/pprof/goroutine?debug=2

# Execution trace (5 seconds)
curl -o trace.out http://localhost:9090/debug/pprof/trace?seconds=5
go tool trace trace.out
```

## Alerting Examples

Example Prometheus alerting rules:

```yaml
groups:
  - name: mirror-crates
    rules:
      - alert: HighErrorRate
        expr: rate(crates_processed_total{result="error"}[5m]) > 10
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High error rate in crate downloads"
          description: "Error rate is {{ $value }} errors/sec"

      - alert: DownloadStalled
        expr: rate(crates_processed_total[5m]) == 0
        for: 10m
        labels:
          severity: critical
        annotations:
          summary: "Crate download has stalled"
          description: "No progress in the last 10 minutes"

      - alert: HighRetryRate
        expr: rate(crates_download_retries_total[5m]) / rate(crates_download_requests_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High retry rate detected"
          description: "{{ $value | humanizePercentage }} of requests are retrying"
```

## Best Practices

1. **Set appropriate scrape intervals** - 15s is usually sufficient; faster intervals add overhead
2. **Monitor inflight counts** - If consistently at max concurrency, consider increasing `-concurrency`
3. **Watch retry rates** - High retries may indicate rate limiting or network issues
4. **Use the JSON endpoint for scripts** - Simpler parsing than Prometheus format
5. **Profile long-running jobs** - Use pprof periodically to catch memory leaks or CPU hotspots
