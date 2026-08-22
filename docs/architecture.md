# Architecture

## Overview

BCRL is a Go controller that drives a synthetic web application (React SPA +
Go backend + SQLite) under controlled load from seven browser/HTTP execution
architectures, collecting raw performance data per run.

```
Controller (cmd/bench)
   |
Scheduler (bounded worker pool)
   |
Worker (per physical slot)
   |-- http      : HTTP executor (no browser)
   |-- headed    : one Chromium per task (scenario A)
   |-- headless  : one headless Chromium per task (scenario B)
   |-- persistent-contexts : one browser + context pool per worker (C)
   |-- cdp       : raw spawn + CDP connect + context pool (D)
   |-- hybrid    : HTTP steps + browser steps, shared session (F)
   |
Scenario -> Execution Engine
   |
Telemetry (system sampler, process monitor)
   |
Results (results/raw/<run-id>/)
```

## Key packages

| Package | Responsibility |
|---|---|
| `cmd/bench` | CLI: version, validate, list-scenarios, run, quick, sweep, summarize, report, status, cleanup |
| `internal/config` | YAML config with defaults and validation |
| `internal/workflow` | Step model, 4 workflows, HTTP executor |
| `internal/browser` | Playwright page executor, per-task browser worker |
| `internal/contexts` | Persistent browser + isolated context pool |
| `internal/cdp` | Raw Chromium spawn + DevToolsActivePort + CDP connect |
| `internal/hybrid` | Transport routing with shared session state |
| `internal/scheduler` | Bounded pool + fixed/ramp/step/steady/spike driver |
| `internal/metrics` | Latency series + counters recorder; system sampler |
| `internal/process` | OS process-tree snapshots |
| `internal/results` | Raw files, metadata, summary, CSV writers |
| `internal/saturation` | Threshold evaluation for sweeps |
| `internal/buildinfo` | Git commit + config hash + versions |
| `internal/exporter` | Prometheus /metrics endpoint |
| `target/backend` | Synthetic Go API + WebSocket + SQLite |
| `target/frontend` | Synthetic React SPA |

## Concurrency model (spec section 25)

Logical concurrency (tasks in flight) is decoupled from physical concurrency
(worker slots). The scheduler pool is sized by `browser_worker_limit` for
browser scenarios and `http_worker_limit` for HTTP; logical tasks queue on the
pool. The driver runs a closed loop: each slot submits a task, waits for
completion, and re-submits until the measurement window ends.

## Timing (spec section 10)

The driver runs warmup, resets the recorder at the measurement boundary, runs
the measurement window, then drains in-flight tasks. Warmup samples never
enter the summary. Setup-phase series (browser launch, context creation, CDP
connect) are preserved across the reset.

## Result artifacts

Each run writes to `results/raw/<run-id>/`:
`metadata.json`, `system_metrics.csv`, `process_metrics.csv`,
`browser_metrics.csv`, `hybrid_metrics.csv`, `task_metrics.csv`,
`summary.json`. Sweeps additionally write `results/sweeps/<ts>.json`.
