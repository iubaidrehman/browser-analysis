# Verifies the host environment before running a large benchmark (spec section 33).
# Usage: powershell -File scripts/environment-check.ps1

$ErrorActionPreference = 'Continue'

function Check-Command($name, $test) {
    $v = & $test 2>$null
    if ($LASTEXITCODE -eq 0 -and $v) { Write-Output ("[OK]   {0}  {1}" -f $name, $v) }
    else { Write-Output ("[MISS] {0}" -f $name) }
}

Write-Output "=== BCRL environment check ==="
Write-Output ("OS:   {0}" -f [System.Environment]::OSVersion.VersionString)
Write-Output ("CPU:  {0} cores" -f $env:NUMBER_OF_PROCESSORS)
$ram = [math]::Round((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory / 1GB, 1)
Write-Output ("RAM:  {0} GB" -f $ram)

Check-Command "go"    { go version }
Check-Command "node"  { node --version }
Check-Command "git"   { git --version }

# Playwright browser cache
$pw = Join-Path $env:LOCALAPPDATA "ms-playwright"
if (Test-Path $pw) {
    Write-Output ("[OK]   playwright cache  {0}" -f $pw)
} else {
    Write-Output "[MISS] playwright cache  (run: go run github.com/mxschmitt/playwright-go/cmd/playwright install chromium)"
}

# Docker (optional)
Check-Command "docker" { docker --version }

# Disk space
$disk = Get-PSDrive -Name $env:SystemDrive.TrimEnd(':') -ErrorAction SilentlyContinue
if ($disk) {
    Write-Output ("[OK]   free disk  {0:N1} GB on {1}" -f ($disk.Free / 1GB), $env:SystemDrive)
}

# Backend reachability (optional)
try {
    $r = Invoke-WebRequest -UseBasicParsing -TimeoutSec 2 http://localhost:8080/api/products
    Write-Output ("[OK]   backend on :8080 (status {0})" -f $r.StatusCode)
} catch {
    Write-Output "[WARN] backend not running on :8080 (start it before benchmarking)"
}
