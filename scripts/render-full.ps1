# Render completo de 12 minutos (36 clips x 20 s) con keep-awake y verificacion.
param(
    [string]$Case = "",
    [string]$OutDir = "D:\panteon-full12",
    [double]$ClipSeconds = 20,
    [int]$ClipCopies = 4,
    [int]$Fps = 60,
    [int]$Width = 1920,
    [int]$Height = 1080,
    [string]$Timeout = "24h"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

if (-not $Case) {
    $candidate = Get-ChildItem "out\final\cases\*.json" -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $candidate) {
        throw "No hay engineering_case. Ejecuta primero: panteon demo --out out/final"
    }
    $Case = $candidate.FullName
}

Write-Host "Compilando panteon..."
go build -o panteon.exe ./engine/cmd/panteon
if ($LASTEXITCODE -ne 0) { throw "go build fallo con codigo $LASTEXITCODE" }

$arguments = @(
    "produce",
    "--case", $Case,
    "--out", $OutDir,
    "--clip-seconds", $ClipSeconds,
    "--clip-copies", $ClipCopies,
    "--fps", $Fps,
    "--width", $Width,
    "--height", $Height,
    "--encoder", "ffmpeg",
    "--renderer", "raster",
    "--timeout", $Timeout,
    "--keep-awake"
)

Write-Host "Renderizando $ClipCopies copias de $ClipSeconds s a ${Width}x${Height}@${Fps}..."
& .\panteon.exe @arguments
if ($LASTEXITCODE -ne 0) { throw "El render fallo con codigo $LASTEXITCODE" }

$manifestPath = Join-Path $OutDir "run_manifest.json"
if (Test-Path $manifestPath) {
    $manifest = Get-Content $manifestPath -Raw | ConvertFrom-Json
    Write-Host "Clips: $($manifest.clips.Count) · Frames: $($manifest.frame_count) · Rigor: $($manifest.rigor.score)"
}

$master = Join-Path $OutDir "master.mp4"
if (Test-Path $master) {
    $info = Get-Item $master
    Write-Host "MP4: $($info.FullName) ($([math]::Round($info.Length/1MB,2)) MB)"
}

$ffprobe = Join-Path $root "tools\ffmpeg\ffprobe.exe"
if (-not (Test-Path $ffprobe)) {
    $ffprobe = (Get-Command ffprobe -ErrorAction SilentlyContinue).Source
}
if ($ffprobe -and (Test-Path $master)) {
    & $ffprobe -v error -show_entries format=duration,size -show_entries stream=codec_name,width,height,r_frame_rate -of json $master
}
