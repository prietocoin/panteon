# Descarga FFmpeg portable dentro del proyecto (sin instalar en el sistema).
param(
    [string]$Destination = (Join-Path $PSScriptRoot "..\tools\ffmpeg"),
    [string]$Url = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n8.1-latest-win64-gpl-shared-8.1.zip"
)

$ErrorActionPreference = "Stop"
$Destination = [System.IO.Path]::GetFullPath($Destination)

if (Test-Path (Join-Path $Destination "bin\ffmpeg.exe")) {
    Write-Host "FFmpeg ya está en $Destination"
    exit 0
}

$temporary = Join-Path ([System.IO.Path]::GetTempPath()) ("elpanteon-ffmpeg-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $temporary | Out-Null
$archive = Join-Path $temporary "ffmpeg.zip"

try {
    Write-Host "Descargando FFmpeg portable..."
    curl.exe -L --fail --retry 3 -o $archive $Url
    if ($LASTEXITCODE -ne 0) { throw "curl falló con código $LASTEXITCODE" }

    Write-Host "Extrayendo..."
    Expand-Archive -Path $archive -DestinationPath (Join-Path $temporary "extract") -Force
    $binary = Get-ChildItem -Path (Join-Path $temporary "extract") -Filter "ffmpeg.exe" -Recurse | Select-Object -First 1
    if (-not $binary) { throw "No se encontró ffmpeg.exe en el archivo" }

    $binDir = $binary.Directory.FullName
    New-Item -ItemType Directory -Path $Destination -Force | Out-Null
    Copy-Item -Path (Join-Path $binDir "*") -Destination $Destination -Recurse -Force
    Write-Host "FFmpeg listo en $Destination"
} finally {
    Remove-Item -Path $temporary -Recurse -Force -ErrorAction SilentlyContinue
}
