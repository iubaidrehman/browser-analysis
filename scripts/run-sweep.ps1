# Runs a benchmark sweep over the given scenarios and concurrency levels.
# Usage:
#   powershell -File scripts/run-sweep.ps1 -Scenarios http,headless,persistent-contexts -Concurrency 1,10,25,50
#   powershell -File scripts/run-sweep.ps1 -Workflow complex -Repetitions 3 -Duration 30

param(
    [string]$Scenarios = "http",
    [string]$Concurrency = "1,10,50,100",
    [string]$Workflow = "complex",
    [int]$Repetitions = 1,
    [int]$Duration = 10,
    [string]$Config = "bench.yaml",
    [string]$Results = "results"
)

Write-Output "=== BCRL sweep ==="
Write-Output ("scenarios: {0}" -f $Scenarios)
Write-Output ("concurrency: {0}" -f $Concurrency)
Write-Output ("workflow: {0}  reps: {1}  duration: {2}s" -f $Workflow, $Repetitions, $Duration)

go run ./cmd/bench sweep `
    --config $Config `
    --scenarios $Scenarios `
    --concurrency $Concurrency `
    --workflow $Workflow `
    --repetitions $Repetitions `
    --results $Results

if ($LASTEXITCODE -ne 0) {
    Write-Error "Sweep failed with exit code $LASTEXITCODE"
    exit $LASTEXITCODE
}
Write-Output "Sweep complete. Run `bench summarize` for a table of results."
