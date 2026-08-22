# Browser Concurrency Research Lab (BCRL)

A controlled, reproducible experimental system that measures the performance
characteristics of different browser automation architectures at increasing
levels of concurrency.

The benchmark operates against a locally hosted synthetic web application. It
does not contact external services.

## Status

**Phase 13** — reproducibility hardening implemented:

- `metadata.json` records the git commit (with `-dirty` marker) and a config
  hash (including the experiment mode) for every run
- Go/Node versions auto-detected; Playwright browser pinned via go.mod
- `bench run --metrics-addr :9091` serves a Prometheus `/metrics` endpoint
  with per-run gauges (throughput, latency, CPU, RAM, failures)

All spec phases are complete except CEF (G), which is explicitly documented
as not-implemented — see `docs/cef.md`. Phases 1-12 cover the repository,
synthetic target, http/headed/headless/persistent-contexts/cdp/hybrid
scenarios, telemetry, process-tree monitoring, and concurrency sweeps.

## Dependencies

| Tool | Version | Purpose |
|---|---|---|
| Go | ≥ 1.26 | controller, backend |
| Node | ≥ 20 | frontend build, SPA runtime |
| Playwright Chromium | pinned by driver | browser scenarios |
| Docker (optional) | any recent | containerized target + telemetry |
| git | any | reproducibility metadata |

Install the Playwright browser:

```sh
go run github.com/mxschmitt/playwright-go/cmd/playwright install chromium
```

## Expected behavior and outcome

- Each `bench run` produces a run directory under `results/raw/` with
  metadata, summary, and raw CSV series; the console prints throughput,
  completed/failed tasks, and latency percentiles.
- `bench sweep` compares each concurrency level to the lowest-level baseline
  and flags **SATURATED** cells that cross degradation thresholds — these are
  research findings, not errors.
- `bench summarize` and `bench report` render tables/markdown from the raw
  data. With no runs, they print "No measurement available." — the system
  never fabricates results.
- The measurement data answers the research question: how startup latency,
  memory, CPU, throughput, P95/P99 latency, failure rate, and scalability
  differ across execution architectures (http, headed, headless,
  persistent-contexts, cdp, hybrid) at concurrency 1–1000.
- Use cases: engineering research on browser-automation architecture
  trade-offs, capacity planning, and comparison of process-per-task vs
  context-reuse vs CDP-control strategies under load.

## Quick start

### Local development

Start the backend:

```sh
cd target/backend
go run . -db /tmp/target.db
```

Start the frontend dev server (proxies `/api` and `/ws` to the backend):

```sh
cd target/frontend
npm install --include=dev
npm run dev
```

Open http://localhost:5173 — the SPA signs itself into a synthetic session,
and the footer shows live WebSocket events.

### Running a benchmark

With the backend running, execute the quick benchmark matrix (concurrency
1/10/50/100, complex workflow):

```sh
go run ./cmd/bench quick --config bench.yaml
```

Or a single run:

```sh
go run ./cmd/bench run --config bench.yaml --concurrency 50 --duration 10
```

Summarize results:

```sh
go run ./cmd/bench summarize
```

Run a browser scenario (requires Playwright Chromium installed; see
`go run github.com/mxschmitt/playwright-go/cmd/playwright install chromium`):

```sh
go run ./cmd/bench run --config bench.yaml --scenario headless --concurrency 2
go run ./cmd/bench run --config bench.yaml --scenario headed --concurrency 2
```

On Windows, run the backend as a compiled binary (`go build -o backend.exe .`)
rather than `go run`, which can be flaky under benchmark load.

### Docker

```sh
docker compose up --build
```

- Frontend: http://localhost:8081
- Backend API: http://localhost:8080
- Optional Postgres, Prometheus, and Grafana: `docker compose --profile telemetry up`

## Full analysis procedure

For the complete end-to-end run (environment check → validation → sweep →
report → cleanup), follow **[docs/benchmark-protocol.md](docs/benchmark-protocol.md)**.
It is the canonical procedure; the quick-start above covers the essentials.

## Synthetic target

The target SPA is fully offline. The backend supports configurable artificial
latency, payload size, and CPU workload via environment variables:

| Variable | Default | Purpose |
| --- | --- | --- |
| `TARGET_ADDR` | `:8080` | listen address |
| `TARGET_DB` | `target.db` | SQLite path |
| `TARGET_API_LATENCY_MS` | `0` | artificial delay per API call |
| `TARGET_PAYLOAD_KB` | `4` | random payload appended to product responses |
| `TARGET_JS_WORKLOAD_UNITS` | `0` | CPU checksum work per checkout |
| `TARGET_SESSION_TTL_SECONDS` | `3600` | session validity window |

## Routes and API

SPA routes: `/login`, `/home`, `/product/:id`, `/cart`, `/checkout`, `/result`.

API: `POST /api/session`, `GET /api/products`, `GET /api/products/:id`,
`POST /api/cart`, `GET /api/cart`, `POST /api/checkout`, `GET /api/order/:id`,
plus the WebSocket endpoint `/ws/events`.

The SPA exercises cookies, localStorage (session id), React state, WebSocket
events, and async API requests — the storage surface the benchmark workflows
rely on.

## Tests

```sh
# Backend unit + integration tests
cd target/backend && go test ./...

# Frontend typecheck + production build
cd target/frontend && npm run build
```

## Layout

- `cmd/bench` — controller CLI entrypoint
- `internal/` — config, logging, results, scenarios (benchmark internals)
- `target/frontend` — synthetic React SPA
- `target/backend` — synthetic Go API + WebSocket + SQLite
- `target/database` — database schema/notes
- `scenarios/` — scenario YAML presets
- `scripts/` — environment-check, cleanup, run-sweep helpers
- `docs/` — architecture, methodology, benchmark protocol, reproducibility, CEF
- `telemetry/` — Prometheus configuration

## Documentation

- `docs/architecture.md` — component and data-flow overview
- `docs/methodology.md` — measurement model and known limitations
- `docs/benchmark-protocol.md` — the reproducible run procedure
- `docs/reproducibility.md` — pinning and clean-machine reproduction
- `docs/cef.md` — why scenario G is not implemented

## License

See [LICENSE](LICENSE).
