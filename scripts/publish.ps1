# publish.ps1 builds and publishes a release.
Set-StrictMode -Version 2.0
$ErrorActionPreference="Stop"

$PublishScript="$(go env GOPATH)\src\github.com\pulumi\home\scripts\publish.ps1"
$BuildSdkScript="$(go env GOPATH)\src\github.com\pulumi\sdk\scripts\build-sdk.ps1"


if (!(Test-Path $PublishScript)) {
    Write-Error "Missing publish script at $PublishScript"
}

$ReleaseInfo=& $PSScriptRoot\make_release.ps1

$PublishTargets=${ReleaseInfo}.Targets
& $PublishScript $ReleaseInfo.ArchivePath "pulumi/windows/amd64" @PublishTargets

Remove-Item -Force $ReleaseInfo.ArchivePath

$Version=& $PSScriptRoot\get-version.ps1
& $BuildSdkScript $Version $(get rev-parse HEAD)
