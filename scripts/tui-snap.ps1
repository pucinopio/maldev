# tui-snap.ps1 — render a license-manager TUI view to PNG via charmbracelet/freeze.
#
# Usage:
#   .\scripts\tui-snap.ps1 -View dashboard
#   .\scripts\tui-snap.ps1 -View licenses -Width 160 -Height 48
#   .\scripts\tui-snap.ps1 -All
#
# Output: ignore\snapshots\<view>.png
#
# Prerequisites:
#   go install github.com/charmbracelet/freeze@latest
#   .\make snap                     # builds bin\tui-snap.exe via the Makefile
#   (or)  go build -o bin\tui-snap.exe .\cmd\tui-snap

param(
    [string]$View = "dashboard",
    [int]$Width = 144,
    [int]$Height = 44,
    [string]$Seed = "",
    [switch]$All
)

$ErrorActionPreference = "Stop"

$allViews = @("dashboard","licenses","issuers","recipients","identities","revocation","servers","audit","settings")

function Resolve-Tool {
    param([string]$Name)
    $c = Get-Command $Name -ErrorAction SilentlyContinue
    if ($c) { return $c.Source }
    $gopath = (& go env GOPATH).Trim()
    $candidate = Join-Path $gopath "bin\$Name.exe"
    if (Test-Path $candidate) { return $candidate }
    throw "$Name not found. Install with: go install github.com/charmbracelet/$Name@latest"
}

$freezeBin = Resolve-Tool freeze

# Use the prebuilt binary if available — falls back to `go run`.
$tuiSnapBin = Join-Path $PSScriptRoot "..\bin\tui-snap.exe"
$useBinary  = Test-Path $tuiSnapBin

function Render-One {
    param([string]$v)
    $seedPath = if ($Seed) { $Seed } else { "scripts\tui-snap-seeds\$v.json" }
    $seedArgs = @()
    if (Test-Path $seedPath) { $seedArgs = @("-seed", $seedPath) }
    $out = "ignore\snapshots\$v.png"
    New-Item -ItemType Directory -Force -Path "ignore\snapshots" | Out-Null

    $tmpAnsi = "ignore\snapshots\$v.ansi"
    $svgOut  = "ignore\snapshots\$v.svg"
    if ($useBinary) {
        & $tuiSnapBin -view $v -width $Width -height $Height @seedArgs | Out-File -FilePath $tmpAnsi -Encoding utf8 -NoNewline
    } else {
        & go run ./cmd/tui-snap -view $v -width $Width -height $Height @seedArgs | Out-File -FilePath $tmpAnsi -Encoding utf8 -NoNewline
    }

    # SVG output — freeze's PNG path crashes on Windows (v0.2.2 GC bug).
    # SVG renders identically in any browser, scales perfectly, faster.
    & $freezeBin $tmpAnsi -l ansi `
        --output $svgOut `
        --window `
        --margin 10 `
        --padding 20 `
        --font.family "Cascadia Code" `
        --font.size 14 `
        --theme "dracula" 2>&1 | Out-Null
    Remove-Item $tmpAnsi -ErrorAction SilentlyContinue

    if (Test-Path $svgOut) {
        Write-Host "wrote $svgOut  ($([int]((Get-Item $svgOut).Length/1024)) KB)"
    } else {
        Write-Host "FAILED: $v" -ForegroundColor Red
        return
    }

    # Optional PNG conversion via headless Chrome (auto-detected).
    $chrome = $null
    foreach ($c in @(
        "C:\Program Files\Google\Chrome\Application\chrome.exe",
        "C:\Program Files (x86)\Google\Chrome\Application\chrome.exe",
        "C:\Program Files\Microsoft\Edge\Application\msedge.exe"
    )) {
        if (Test-Path $c) { $chrome = $c; break }
    }
    if ($chrome) {
        $pngOut  = $svgOut -replace '\.svg$','.png'
        $absSvg  = (Resolve-Path $svgOut).Path -replace '\\','/'
        $absPng  = (Join-Path (Get-Location) $pngOut) -replace '\\','/'
        & $chrome --headless --disable-gpu --no-sandbox --hide-scrollbars `
            "--screenshot=$absPng" `
            "--window-size=1600,1400" `
            "file:///$absSvg" 2>&1 | Out-Null
        if (Test-Path $pngOut) {
            Write-Host "wrote $pngOut  ($([int]((Get-Item $pngOut).Length/1024)) KB)"
        }
    }
}

if ($All) {
    foreach ($v in $allViews) { Render-One $v }
} else {
    Render-One $View
}
