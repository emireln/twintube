# TwinTube — stop local dev stack (Windows)
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$StateDir = Join-Path $Root ".local"
$StateFile = Join-Path $StateDir "dev-state.json"

Set-Location $Root

function Write-Info($msg) { Write-Host "[TwinTube] $msg" -ForegroundColor Cyan }
function Write-Ok($msg) { Write-Host "[TwinTube] $msg" -ForegroundColor Green }
function Write-Warn($msg) { Write-Host "[TwinTube] $msg" -ForegroundColor Yellow }

$stoppedSomething = $false

if (Test-Path $StateFile) {
    try {
        $state = Get-Content $StateFile -Raw | ConvertFrom-Json

        if ($state.mode -eq "docker-compose") {
            if (Get-Command docker -ErrorAction SilentlyContinue) {
                Write-Info "Stopping Docker Compose stack ..."
                docker compose down
                $stoppedSomething = $true
            }
        } else {
            if ($state.appPid) {
                $proc = Get-Process -Id $state.appPid -ErrorAction SilentlyContinue
                if ($proc) {
                    Write-Info "Stopping Go server (PID $($state.appPid)) ..."
                    Stop-Process -Id $state.appPid -Force -ErrorAction SilentlyContinue
                    $stoppedSomething = $true
                }
            }
            if ($state.postgresStarted -and (Get-Command docker -ErrorAction SilentlyContinue)) {
                Write-Info "Stopping PostgreSQL container ..."
                docker compose stop postgres
                $stoppedSomething = $true
            }
        }
    } catch {
        Write-Warn "Could not read state file; trying fallback cleanup."
    }

    Remove-Item $StateFile -Force -ErrorAction SilentlyContinue
}

# Fallback: kill any go process running twintube main from this repo
Get-CimInstance Win32_Process -Filter "Name = 'go.exe'" -ErrorAction SilentlyContinue | ForEach-Object {
    if ($_.CommandLine -match [regex]::Escape($Root)) {
        Write-Info "Stopping leftover go process (PID $($_.ProcessId)) ..."
        Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
        $stoppedSomething = $true
    }
}

# Fallback: twintube.exe if built locally
Get-CimInstance Win32_Process -Filter "Name = 'twintube.exe'" -ErrorAction SilentlyContinue | ForEach-Object {
    if ($_.ExecutablePath -and ($_.ExecutablePath -like "$Root*")) {
        Write-Info "Stopping twintube.exe (PID $($_.ProcessId)) ..."
        Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
        $stoppedSomething = $true
    }
}

# Fallback: docker compose if containers still up
if (Get-Command docker -ErrorAction SilentlyContinue) {
    $running = docker compose ps --services --filter "status=running" 2>$null
    if ($LASTEXITCODE -eq 0 -and $running) {
        Write-Info "Stopping running Docker Compose services ..."
        docker compose down
        $stoppedSomething = $true
    }
}

if ($stoppedSomething) {
    Write-Ok "TwinTube local stack stopped."
} else {
    Write-Warn "Nothing was running (or already stopped)."
}
