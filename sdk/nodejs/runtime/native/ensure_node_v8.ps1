Set-StrictMode -Version 2.0
$ErrorActionPreference = "Stop"

$NodeBase="$PSScriptRoot\third_party\node"
$NodeTarget="$NodeBase\node-$(node -p "process.version")"

if (Test-Path "$NodeTarget") {
    Write-Output "Node.js/V8 internal sources and headers download, as they already exist"
} else {
    $NodeDistro=node -p "process.release.sourceUrl"
    Write-Output "Downloading Node.js/V8 internal sources and headers from $NodeDistro..."
    $NodeTarball=[System.IO.Path]::GetTempFileName()
    Invoke-WebRequest -Uri "$NodeDistro" -OutFile "$NodeTarball"

    # Unfortunately, 7-zip can't extract a tgz in a single gesture
    $NodeTarPath=Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
    New-Item -ItemType Directory -Path "$NodeTarPath"
    7z x -o"$NodeTarPath" "$NodeTarball"
    New-Item -ItemType Directory -Path "$NodeBase" 
    7z x -o"$NodeBase" (Join-Path "$NodeTarPath" "node-$(node -p "process.version").tar")
    Remove-Item -Force -Recurse "$NodeTarPath"
    Remove-Item -Force "$NodeTarball"
}

Write-Output "Done; $NodeTarget is fully populated."
