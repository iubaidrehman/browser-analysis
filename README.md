# Browser Concurrency Research Lab (BCRL)

A controlled, reproducible experimental system that measures the performance
characteristics of different browser automation architectures at increasing
levels of concurrency.

The benchmark operates against a locally hosted synthetic web application. It
does not contact external services.

## Status

**Phase 10** — process-tree monitoring implemented:

- OS process table snapshots every 5s (PID, PPID, name, CPU, RSS, threads)
  via Win32 API, persisted to `process_metrics.csv`
- Captures the controller → Chromium → renderer/GPU/utility topology so
  logical tasks are distinguishable from OS processes
- Per-process CPU normalized to the system denominator; elevated/system
  processes flagged with `access_denied`

Phases 3-9 (http, headed, headless, persistent-contexts, cdp, hybrid,
telemetry) remain implemented. CEF (G) is not yet implemented.

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
- `scenarios/` — scenario YAML presets (planned)
- `docs/` — architecture and methodology documentation
- `telemetry/` — Prometheus configuration

## License

See [LICENSE](LICENSE).
