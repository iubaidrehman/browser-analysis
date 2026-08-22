# Runs the full benchmark matrix: http, persistent-contexts, headless at
# concurrency 1, 5, 10, 25, 50, 100, 250, 500, 750, 1000 (3 reps each) with
# adaptive safety.
# Prerequisite: the target backend must be running (see target/backend).
# Usage:
#   powershell -File scripts/run-full-matrix.ps1
#   powershell -File scripts/run-full-matrix.ps1 -Concurrency 1,10,50,100 -Repetitions 3 -MaxRSSGB 14

param(
    [string]$Concurrency = "1,5,10,25,50,100,250,500,750,1000",
    [int]$Repetitions = 3,
    [float]$MaxRSSGB = 14,
    [string]$Config = "bench.yaml",
    [string]$Results = "results"
)

Write-Output "=== BCRL full matrix sweep ==="
Write-Output ("scenarios: http, persistent-contexts, headless")
Write-Output ("concurrency: {0}" -f $Concurrency)
Write-Output ("repetitions: {0}   max-rss ceiling: {1} GB" -f $Repetitions, $MaxRSSGB)

go run ./cmd/bench sweep `
    --config $Config `
    --scenarios http,persistent-contexts,headless `
    --concurrency $Concurrency `
    --repetitions $Repetitions `
    --max-rss-gb $MaxRSSGB `
    --results $Results

if ($LASTEXITCODE -ne 0) {
    Write-Error "Full matrix sweep failed with exit code $LASTEXITCODE"
    exit $LASTEXITCODE
}

Write-Output "Full matrix sweep complete."
Write-Output "Generate all reports with:  powershell -File scripts/generate-reports.ps1"
