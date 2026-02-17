# Build Windows installer using PowerShell (no make required)
# Usage: .\build\windows\build.ps1 [-Version "1.0.0"]

param(
    [string]$Version = "1.0.0"
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Resolve-Path "$PSScriptRoot\..\.."
$Dist = Join-Path $ProjectRoot "dist"
$WinDist = Join-Path $Dist "windows-amd64"

Write-Host "=== VPN Client Windows Build ===" -ForegroundColor Cyan
Write-Host "Version: $Version"

# ─── Step 1: Build the executable ───
Write-Host "`n[1/3] Building vpn-client.exe..." -ForegroundColor Yellow
New-Item -ItemType Directory -Force -Path $WinDist | Out-Null

Push-Location $ProjectRoot
try {
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"
    
    go build -ldflags "-H=windowsgui -s -w -X main.Version=$Version" `
        -o "$WinDist\vpn-client.exe" ./cmd/vpn-client
    
    if ($LASTEXITCODE -ne 0) {
        throw "Go build failed with exit code $LASTEXITCODE"
    }
} finally {
    Pop-Location
    Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
    Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
}

Write-Host "  Built: $WinDist\vpn-client.exe" -ForegroundColor Green

# ─── Step 2: Check for Inno Setup ───
Write-Host "`n[2/3] Looking for Inno Setup..." -ForegroundColor Yellow

$ISSCPaths = @(
    "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
    "$env:ProgramFiles\Inno Setup 6\ISCC.exe",
    "${env:ProgramFiles(x86)}\Inno Setup 5\ISCC.exe"
)

$ISCC = $null
foreach ($path in $ISSCPaths) {
    if (Test-Path $path) {
        $ISCC = $path
        break
    }
}

if (-not $ISCC) {
    # Try to find in PATH
    $ISCC = Get-Command "iscc" -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
}

if (-not $ISCC) {
    Write-Host "  Inno Setup not found!" -ForegroundColor Red
    Write-Host "  Download from: https://jrsoftware.org/isdl.php" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "  Binary is available at: $WinDist\vpn-client.exe" -ForegroundColor Yellow
    Write-Host "  You can run the Inno Setup compiler manually:" -ForegroundColor Yellow
    Write-Host "  iscc build\windows\installer.iss" -ForegroundColor White
    exit 1
}

Write-Host "  Found: $ISCC" -ForegroundColor Green

# ─── Step 3: Build the installer ───
Write-Host "`n[3/3] Creating installer..." -ForegroundColor Yellow

& $ISCC "/DMyAppVersion=$Version" "$ProjectRoot\build\windows\installer.iss"

if ($LASTEXITCODE -ne 0) {
    throw "Inno Setup failed with exit code $LASTEXITCODE"
}

$InstallerPath = Join-Path $Dist "vpn-client-$Version-windows-setup.exe"

Write-Host "`n=== Build Complete ===" -ForegroundColor Green
Write-Host "Executable: $WinDist\vpn-client.exe"
Write-Host "Installer:  $InstallerPath"
