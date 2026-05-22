[CmdletBinding()]
param(
    [switch]$RemoveFromPath
)

$ErrorActionPreference = "Stop"

if (-not $env:LOCALAPPDATA) {
    throw "LOCALAPPDATA no esta definido. Ejecuta este script en Windows PowerShell."
}

$TargetDir = Join-Path $env:LOCALAPPDATA "Programs\DevHerd"
$TargetBin = Join-Path $TargetDir "devherd.exe"

if (Test-Path $TargetBin) {
    Remove-Item $TargetBin -Force
    Write-Host "devherd eliminado de $TargetBin"
}
else {
    Write-Host "devherd no esta instalado en $TargetBin"
}

if ($RemoveFromPath) {
    $currentUserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($currentUserPath) {
        $entries = $currentUserPath -split ";" | Where-Object {
            $_ -and ($_.TrimEnd([char]'\') -ine $TargetDir.TrimEnd([char]'\'))
        }
        [Environment]::SetEnvironmentVariable("Path", ($entries -join ";"), "User")
        Write-Host "PATH de usuario actualizado."
    }
}
