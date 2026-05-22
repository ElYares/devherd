[CmdletBinding()]
param(
    [switch]$AddToPath
)

$ErrorActionPreference = "Stop"

$RootDir = Split-Path -Parent $PSScriptRoot
if (-not $env:LOCALAPPDATA) {
    throw "LOCALAPPDATA no esta definido. Ejecuta este script en Windows PowerShell."
}

$TargetDir = Join-Path $env:LOCALAPPDATA "Programs\DevHerd"
$TargetBin = Join-Path $TargetDir "devherd.exe"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go no esta en PATH. Instala Go y vuelve a ejecutar este script."
}

New-Item -ItemType Directory -Force -Path $TargetDir | Out-Null

Push-Location $RootDir
try {
    go build -o $TargetBin ./cmd/devherd
}
finally {
    Pop-Location
}

Write-Host "devherd instalado en $TargetBin"

if ($AddToPath) {
    $currentUserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $entries = @()
    if ($currentUserPath) {
        $entries = $currentUserPath -split ";"
    }

    $alreadyPresent = $false
    foreach ($entry in $entries) {
        if ($entry -and ($entry.TrimEnd([char]'\') -ieq $TargetDir.TrimEnd([char]'\'))) {
            $alreadyPresent = $true
            break
        }
    }

    if (-not $alreadyPresent) {
        $newUserPath = if ($currentUserPath) { "$currentUserPath;$TargetDir" } else { $TargetDir }
        [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
        $env:Path = "$env:Path;$TargetDir"
        Write-Host "PATH de usuario actualizado. Abre una terminal nueva para usar devherd globalmente."
    }
    else {
        Write-Host "PATH de usuario ya contiene $TargetDir"
    }
}
else {
    Write-Host "Para agregarlo al PATH, vuelve a ejecutar: .\scripts\install-windows.ps1 -AddToPath"
}
