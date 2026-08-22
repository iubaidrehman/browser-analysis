# Cleans leftover benchmark state (spec section 30): kills stray backend and
# Chromium processes. Run results are preserved.
# Usage: powershell -File scripts/cleanup.ps1

Write-Output "=== BCRL cleanup ==="

# Kill stray backend.exe and bench.exe processes.
foreach ($name in @("backend", "bench")) {
    $procs = Get-Process -Name $name -ErrorAction SilentlyContinue
    if ($procs) {
        Write-Output ("Stopping {0} process(es): {1}" -f $name, ($procs.Id -join ", "))
        $procs | Stop-Process -Force
    }
}

# Kill stray Chromium processes spawned by the benchmark.
foreach ($name in @("chrome", "chrome_headless_shell", "headless_shell")) {
    $procs = Get-Process -Name $name -ErrorAction SilentlyContinue
    if ($procs) {
        Write-Output ("Stopping {0} process(es): {1}" -f $name, ($procs.Id -join ", "))
        $procs | Stop-Process -Force
    }
}

# Remove stray SQLite smoke databases left in results/raw.
Get-ChildItem -Path results/raw -Filter ".phase*.db*" -Force -ErrorAction SilentlyContinue |
    Remove-Item -Force -ErrorAction SilentlyContinue

Write-Output "Cleanup complete. No orphan browser/backend processes remain."
