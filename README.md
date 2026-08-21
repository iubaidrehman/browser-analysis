# Browser Concurrency Research Lab (BCRL)

A controlled, reproducible experimental system that measures the performance
characteristics of different browser automation architectures at increasing
levels of concurrency.

The benchmark operates against a locally hosted synthetic web application. It
does not contact external services.

## Status

**Phase 2** — synthetic target application:

- React + TypeScript + Vite SPA with the full route set and storage layers
- Go backend with REST API, WebSocket events, and SQLite
- Docker Compose for frontend, backend, and optional observability

Benchmark scenarios (Phases 3+) are not yet implemented.

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
