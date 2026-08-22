# Reproducibility

## Recorded per run

Every run's `metadata.json` records (spec section 16, 34):

- `run_id`, `scenario`, `workflow`, `concurrency`, timing windows
- `environment` — OS, architecture, CPU cores
- `software` — Go and Node versions
- `configuration` — browser mode, headless, context count, worker count
- `git_commit` — the exact commit the benchmark ran from, suffixed `-dirty`
  when the working tree had uncommitted changes
- `config_hash` — SHA-256 prefix of the effective config plus the experiment
  mode

## Pinning

- Go dependencies are pinned in `go.mod` (including the Playwright driver
  version, which pins the Chromium revision via its browser manifest).
- The synthetic target's frontend pins React/Vite/TypeScript in
  `target/frontend/package-lock.json`.
- Browser binaries are installed by the pinned Playwright driver into the
  per-user cache (`%LOCALAPPDATA%\ms-playwright`); record the installed
  revision from `browser_metrics.csv`/metadata for exactness.

## Clean-machine reproduction

On a fresh machine with Go ≥ 1.26, Node ≥ 20, and git:

```sh
git clone <repo-url>
cd browser-analysis
# Install Playwright browsers pinned by the driver
go run github.com/mxschmitt/playwright-go/cmd/playwright install chromium
# Build the synthetic target backend and start it
cd target/backend && go build -o backend.exe . && ./backend.exe -db target.db &
# Run a benchmark
cd ../.. && go run ./cmd/bench run --config bench.yaml --scenario http --concurrency 50
```

The `git_commit` in the resulting `metadata.json` identifies the exact
revision, so any two runs from the same commit + config hash are directly
comparable.

## Known limitations

- Disk/network counters are not yet collected on Windows (zeroed in
  `system_metrics.csv`).
- The POSIX collectors for system/process metrics are stubs; the lab is
  currently Windows-first.
- Chromium revision is pinned by the Playwright driver version, not an
  explicit browser build number in this repo.
