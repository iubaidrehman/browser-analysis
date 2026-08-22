# Generates ALL benchmark reports from the runs on disk in one command.
# Usage:
#   powershell -File scripts/generate-reports.ps1
#   powershell -File scripts/generate-reports.ps1 -Results results

param(
    [string]$Results = "results"
)

Write-Output "=== BCRL report generation ==="

# Main report: sweep table + runs list + holistic resource/scaling summary.
go run ./cmd/bench report $Results
if ($LASTEXITCODE -ne 0) {
    Write-Error "bench report failed"
    exit $LASTEXITCODE
}

# Console summary (positional results dir).
Write-Output ""
Write-Output "=== Summarize (all runs) ==="
go run ./cmd/bench summarize $Results

Write-Output ""
Write-Output "=== Generated artifacts ==="
Write-Output "Main report:  $Results/report.md"
Write-Output "Detailed:     $Results/reports/resource-report.md"
Write-Output "              $Results/reports/topology-report.md"
Write-Output "              $Results/reports/scaling-report.md"
Write-Output "Summaries:    $Results/summaries/resource-summary.csv"
Write-Output "              $Results/summaries/scaling-summary.csv"
Write-Output "Sweep JSON:   $Results/sweeps/<timestamp>.json"
Write-Output ""
Write-Output "Per-run detail:  go run ./cmd/bench analyze-run --run <RUN_ID>"
Write-Output "                 go run ./cmd/bench resources --run <RUN_ID>"
Write-Output "                 go run ./cmd/bench topology --run <RUN_ID>"
