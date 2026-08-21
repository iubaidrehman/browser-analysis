# Browser Concurrency Research Lab (BCRL)

A controlled, reproducible experimental system that measures the performance
characteristics of different browser automation architectures at increasing
levels of concurrency.

The benchmark operates against a locally hosted synthetic web application. It
does not contact external services.

## Status

**Phase 1** — repository scaffold, Go CLI, configuration, logging, and the
result model. Benchmark execution is not yet implemented.

## Commands

```sh
go run ./cmd/bench version
go run ./cmd/bench list-scenarios
go run ./cmd/bench validate
```

## Layout

- `cmd/bench` — controller CLI entrypoint
- `internal/config` — YAML configuration
- `internal/logging` — structured logging
- `internal/results` — raw result model
- `internal/scenarios` — scenario catalogue
- `target/` — synthetic application (frontend, backend, database)
- `scenarios/` — scenario YAML presets
- `docs/` — architecture and methodology documentation

## License

See [LICENSE](LICENSE).
