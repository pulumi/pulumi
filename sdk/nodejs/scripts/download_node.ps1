Set-StrictMode -Version 2.0
$ErrorActionPreference="Stop"

$NodeArch="x64"
$NodeVersion="6.10.2"
$NodeBase="custom_node\node"
$NodeExe="custom_node\node\node.exe"
$NodeZipName="$NodeBase\node-$NodeVersion-win-$NodeArch.zip"
if (Test-Path $NodeExe) {
    Write-Output "skipping node.js executable download, as it already exists"
} else {
    echo "node.js binary does not exist, downloading..."
    aws s3 cp --only-show-errors "s3://eng.pulumi.com/node/node-$NodeVersion-win-$NodeArch.zip" $NodeZipName
    7z x -o"$NodeBase" $NodeZipName
    Remove-Item -Force $NodeZipName
}