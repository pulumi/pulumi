Set-StrictMode -Version 2.0
$ErrorActionPreference="Stop"

$NodeVersion="v6.10.2"
$NodeBase="custom_node\node"
$NodeExe="custom_node\node\node.exe"
if (Test-Path $NodeExe) {
    Write-Output "skipping node.js executable download, as it already exists"
} else {
    echo "node.js binary does not exist, downloading..."
    aws s3 cp --only-show-errors "s3://eng.pulumi.com/releases/pulumi-node/windows/$NodeVersion.zip" "$NodeBase\$NodeVersion.zip"

    $NodeZipPath = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
    7z x -o$NodeZipPath "$NodeBase\$NodeVersion.zip"
    Copy-Item $NodeZipPath\Release\node.exe $NodeExe

    Remove-Item -Force -Recurse $NodeZipPath
    Remove-Item -Force "$NodeBase\$NodeVersion.zip"
}