# Runs the persistent-context scaling experiment (milestone section 16):
# concurrency 100, 200, 300, 400, 500, 750, 1000 with adaptive safety.
# Usage:
#   powershell -File scripts/run-scaling.ps1
#   powershell -File scripts/run-scaling.ps1 -MaxRSSGB 14
#   powershell -File scripts/run-scaling.ps1 -Concurrency 100,200,300 -Repetitions 3

param(
    [string]$Concurrency = "100,200,300,400,500,750,1000",
    [int]$Repetitions = 1,
    [float]$MaxRSSGB = 14,
    [string]$Config = "scenarios/scaling.yaml",
    [string]$Results = "results"
)

Write-Output "=== BCRL persistent-context scaling sweep ==="
Write-Output ("concurrency: {0}" -f $Concurrency)
Write-Output ("repetitions: {0}   max-rss ceiling: {1} GB" -f $Repetitions, $MaxRSSGB)
Write-Output ("config: {0}" -f $Config)

go run ./cmd/bench sweep `
    --config $Config `
    --scenarios persistent-contexts `
    --concurrency $Concurrency `
    --repetitions $Repetitions `
    --max-rss-gb $MaxRSSGB `
    --results $Results

if ($LASTEXITCODE -ne 0) {
    Write-Error "Scaling sweep failed with exit code $LASTEXITCODE"
    exit $LASTEXITCODE
}

Write-Output "Scaling sweep complete."
Write-Output "Generate the report with:  go run ./cmd/bench report"
