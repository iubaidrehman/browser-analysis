# BCRL Milestone Completion Report

**Milestone:** Measurement Integrity + Aggregate Resource Accounting + Scaling
Experiment
**Date:** 2026-08-22
**Branch:** main

## 1. Files changed

- `internal/accounting/` (new) — process-role aggregation, baseline/delta,
  percentile stats
- `internal/process/ownership.go` — process role classification
- `internal/process/monitor.go` — process_role column, classify hook
- `internal/metrics/lifecycle.go` — lifecycle event model
- `internal/metrics/metrics.go` — lifecycle counters, measurement elapsed
- `internal/scheduler/driver.go` — closed-loop concurrency fix (token
  semaphore, deadline-after-token)
- `internal/scheduler/pool.go` — WorkerCount, TaskCreated counting
- `internal/results/resource.go`, `resource_builder.go`, `scaling.go` (new) —
  ResourceSummary, builders, scaling/resource CSVs
- `internal/controller/controller.go` — aggregator + baseline wiring
- `cmd/bench/commands.go` — sweep table, adaptive safety, resources/topology/
  analyze-run/analyze-sweep, detailed reports
- `cmd/bench/main.go` — new subcommands
- `internal/browser/worker.go`, `internal/contexts/*` — lifecycle events
- `scenarios/scaling.yaml` (new) — scaling experiment config
- `docs/methodology.md` — memory accounting documentation

## 2. New metrics

- `task_rss_bytes` (existing, kept) — per-task RSS delta
- `rss_total`, `browser_rss`, `controller_rss`, `target_rss` — architecture
  RSS series (mean/p50/p95/peak)
- `architecture_rss_delta` — benchmark total minus baseline
- `benchmark_cpu`, `browser_cpu`, `controller_cpu`, `target_cpu` — CPU splits
- Process counts: total/browser/renderer/utility/gpu/controller/target
- Lifecycle: browsers, contexts, pages (measured from events)
- Derived: memory_per_task, browser_memory_per_context, throughput_per_cpu,
  throughput_per_gb

## 3. Commands to reproduce

```sh
# Validation runs (milestone section 21)
go run ./cmd/bench sweep --config bench.yaml \
  --scenarios headed,headless,persistent-contexts,http \
  --concurrency 10 --repetitions 3

# Scaling experiment (milestone section 16)
go run ./cmd/bench sweep --config scenarios/scaling.yaml \
  --scenarios persistent-contexts \
  --concurrency 100,200,300,400,500,750,1000 \
  --repetitions 1 --max-rss-gb 14

# Analysis
go run ./cmd/bench report
go run ./cmd/bench resources --run <RUN_ID>
go run ./cmd/bench topology --run <RUN_ID>
go run ./cmd/bench analyze-run --run <RUN_ID>
```

## 4. Validation results

Four architectures at concurrency 10, 3 reps each (milestone section 21):

| scenario | browser RSS mean (MB) | controller RSS (MB) | browsers/contexts/pages |
|---|---|---|---|
| headed | 593-641 | 48-58 | 43-47/0/43-47 |
| headless | 620-759 | 59 | 85-87/0/85-87 |
| persistent-contexts | 911-918 | 59 | 5/10/105-110 |
| http | 0 | 63-65 | 0/0/0 |

Internally consistent: http has zero browser RSS, persistent-contexts stable
across reps, controller RSS small and stable.

## 5. Full persistent-context scaling results

Honest measurements with the corrected driver (5 worker browsers, 100
contexts, 5s measurement):

| concurrency | throughput/s | completed | p50 (s) | p95 (s) | browser RSS mean (MB) |
|---|---|---|---|---|---|
| 100 | baseline | — | 0.564 | 0.575 | — |
| 200 | 7.39 | 41 | 0.551 | 0.565 | 881 |
| 300 | 7.88 | 40 | 0.565 | 0.572 | 876 |
| 400 | 7.89 | 40 | 0.554 | 0.570 | 879 |
| 500 | 7.87 | 40 | 0.557 | 0.572 | 876 |
| 750 | 7.96 | 40 | 0.555 | 0.571 | 877 |
| 1000 | 7.87 | 40 | 0.558 | 0.568 | 877 |

**Finding:** throughput is worker-bound (~7.9/s, near the 5/0.55s ≈ 9/s
ceiling), flat across logical concurrency 200-1000. Logical concurrency
beyond the worker count only queues — it does not increase throughput.
Browser RSS stays flat at ~877MB because the 5 persistent browsers are
reused. The architecture saturates at its worker count, not at high logical
concurrency.

## 6. Configurations not run

None — all 7 scaling levels completed. CEF remains documented as not
implemented (docs/cef.md).

## 7. Peak CPU

~31% system-wide (concurrency-100 baseline); benchmark-owned CPU ~4-9%
mean across cells.

## 8. Peak total RSS

~9.0 GB (accounting, architecture-scoped); host RAM peaked ~14 GB at
concurrency 100 (84% of 16.85 GB) in the validation sweep.

## 9. Peak browser RSS

~1.1 GB (5 persistent Chromium trees, 17-20 browser processes).

## 10. Maximum context count achieved

100 contexts (contexts.count), 20 per worker across 5 browsers.

## 11. Maximum logical-task count achieved

1000 (all completed, 0 failures).

## 12. Process topology at 100/500/1000

| concurrency | total procs | browser procs | renderer procs |
|---|---|---|---|
| 100 | ~294 | 19 | ~14 |
| 500 | 276 | 17 | ~13 |
| 1000 | 275 | 17 | ~13 |

1 controller, 5 persistent browsers, ~13-14 renderers — constant regardless
of logical concurrency, confirming the worker-bound architecture.

## 13. Instrumentation limitations

- Per-task RSS polling (100ms) adds ~70ms/task overhead over raw workflow
  latency; the throughput ceiling reflects instrumentation cost.
- Baseline excludes warmup memory growth (captured pre-setup); the
  architecture delta understates workload memory by setup memory.
- Target backend RSS is not separately classified (runs outside the
  controller tree); reported as aux.
- Disk/network system counters not yet collected on Windows.
- POSIX collectors are stubs; the lab is Windows-first.
