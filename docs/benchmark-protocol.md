# Benchmark Protocol

## Purpose

A reproducible procedure for measuring how browser automation architectures
behave under concurrency, against a locally hosted synthetic target. Read the
[README](../README.md) first for an overview, dependencies, and expected
outcomes; this document is the step-by-step procedure.

## Prerequisites

- Go ≥ 1.26, Node ≥ 20, git
- Playwright Chromium installed: `go run github.com/mxschmitt/playwright-go/cmd/playwright install chromium`
- Docker optional (frontend/backend containers)

## Environment check

```sh
powershell -File scripts/environment-check.ps1
```

Confirms toolchain, Playwright cache, disk, and backend reachability before a
large run.

## Quick validation (spec section 21)

1. Start the backend: `cd target/backend && go build -o backend.exe . && ./backend.exe -db target.db &`
2. `go run ./cmd/bench quick --config bench.yaml`
   Runs http, headless, persistent-contexts, headed at concurrency 1/10/50/100
   with short timing. Expected: every cell completes with 0 failures and
   `results/raw/<run-id>/` populated.

## Single run (spec section 23)

```sh
go run ./cmd/bench run --config bench.yaml --scenario headless --concurrency 50 --duration 180
```

Expected: a run directory under `results/raw/` containing metadata.json,
summary.json, and the CSV series; the console prints a run summary (tasks,
throughput, latency percentiles).

## Sweep (spec section 22)

```sh
powershell -File scripts/run-sweep.ps1 -Scenarios http,headless,persistent-contexts -Concurrency 1,10,50,100 -Workflow complex -Repetitions 3 -Duration 30
```

or directly:

```sh
go run ./cmd/bench sweep --scenarios http,headless --concurrency 1,10,50,100 --repetitions 3
```

Expected: one run per cell; baseline = lowest concurrency averaged across
repetitions; cells crossing thresholds (CPU>90%, P95>2x baseline, P99>3x,
failure>2%) are flagged SATURATED; verdicts persist to `results/sweeps/`.

## Analysis

```sh
go run ./cmd/bench summarize                      # console table of all runs
go run ./cmd/bench report                         # markdown report (results/report.md + results/reports/*.md)
go run ./cmd/bench resources --run <RUN_ID>       # per-run memory/CPU/process/lifecycle accounting
go run ./cmd/bench topology --run <RUN_ID>        # per-run process topology
go run ./cmd/bench analyze-run --run <RUN_ID>     # deep-dive on one run
go run ./cmd/bench analyze-sweep --sweep <FILE>   # saturation summary across a sweep
```

`bench report` writes the main `results/report.md` plus three detailed
reports under `results/reports/`: `resource-report.md` (RSS by role per run),
`topology-report.md` (process counts per run), and `scaling-report.md`
(throughput/latency/RSS vs concurrency per scenario). Machine-readable
summaries land in `results/summaries/` (`resource-summary.csv`,
`scaling-summary.csv`).

Each run directory under `results/raw/<run-id>/` contains `summary.json`
(throughput, latency percentiles, task RSS) and `resource_summary.json`
(baseline RSS, architecture delta, browser/total RSS series, CPU splits,
process counts, measured browsers/contexts/pages).

## Cleanup (spec section 30)

```sh
powershell -File scripts/cleanup.ps1
```

Kills stray backend/Chromium processes and removes smoke databases. Run
results are preserved.

## Expected behavior

- No measurement: `bench summarize`/`bench report` print "No measurement
  available." — the system never fabricates numbers.
- A run with 0 failures and sane latency percentiles is the healthy outcome;
  saturation flags in sweeps are expected research findings, not errors.
- Cancellation (Ctrl+C): partial results persist, exit code is nonzero.
- No orphan Chromium processes after any run.
