# Methodology

## Benchmark model

The benchmark measures browser automation architectures under concurrency
against a local synthetic target. Every run produces raw artifacts under
`results/raw/<run-id>/`:

- `metadata.json` — run configuration, environment, software versions
- `system_metrics.csv` — host resource samples (1s interval)
- `browser_metrics.csv` — browser launch, context creation, CDP connect
- `hybrid_metrics.csv` — HTTP vs browser step time splits (scenario F)
- `task_metrics.csv` — task/request counters
- `summary.json` — aggregate statistics

## Warmup / measurement / cooldown

The driver runs a warmup phase, resets the recorder at the measurement
boundary, then runs the measurement window. Warmup samples never enter the
primary latency, throughput, or failure statistics (spec section 10).

System telemetry samples are captured across the whole run including warmup;
the CSV keeps per-sample timestamps so analysis can trim to the measurement
window. Summary peak/avg CPU and RAM are computed over all captured samples.

## Latency accounting

Each workflow step records a duration. Step latencies are aggregated into the
summary's latency distribution. Browser launch latency (scenarios A/B/C) and
CDP connect latency (scenario D) are recorded as separate series.

## Known limitations

- **Disk and network counters are not yet collected on Windows** (they are
  zeroed in `system_metrics.csv`). They require performance-counter sampling
  and are deferred.
- **Swap used** on Windows reports the page-file commit charge, which is zero
  on systems without a page file.
- **The POSIX collector is a stub** returning zeroed snapshots; the lab is
  currently Windows-first.
- **CPU% denominators**: `system_metrics.csv` reports host-wide CPU as a
  percent of all cores. `process_metrics.csv` per-process CPU is normalized
  to the same denominator (percent of all cores); both are clamped to
  [0, 100].
- **Elevated/system processes** appear in `process_metrics.csv` with
  `access_denied=true` and zero CPU/RSS because they cannot be opened for
  sampling by a non-elevated process.
- **CEF (scenario G) is not implemented** — it requires a separate CEF
  toolchain and is documented as not-implemented rather than shipping a
  fragile experimental path.

## Neutrality

The benchmark makes no claim that any architecture is faster. Results are
recorded as raw measurements plus aggregate statistics; conclusions are left
to analysis.
