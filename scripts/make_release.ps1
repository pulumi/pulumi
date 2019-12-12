# make_release.ps1 will create a build package ready for publishing.
Set-StrictMode -Version 2.0
$ErrorActionPreference="Stop"

$Root=Join-Path $PSScriptRoot ".."
$PublishDir=New-Item -ItemType Directory -Path "$env:TEMP\$([System.IO.Path]::GetRandomFileName())"
$GitHash=$(git rev-parse HEAD)
$PublishFile="$(Split-Path -Parent -Path $PublishDir)\$GitHash.zip"
$Version = $( & "$PSScriptRoot\get-version.cmd")
$Branch = $(if (Test-Path env:APPVEYOR_REPO_BRANCH) { $env:APPVEYOR_REPO_BRANCH } else { $(git rev-parse --abbrev-ref HEAD) })
$PublishTargets = @($GitHash, $Version, $Branch)

function RunGoBuild($goPackage) {
    $binRoot = New-Item -ItemType Directory -Force -Path "$PublishDir\bin"
    $outputName = Split-Path -Leaf $(go list -f "{{.Target}}" $goPackage)
    go build -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=$Version" -o "$binRoot\$outputName" $goPackage
}

function CopyPackage($pathToModule, $moduleName) {
    $moduleRoot = New-Item -ItemType Directory -Force -Path "$PublishDir\node_modules\$moduleName"
    Copy-Item -Recurse $pathToModule\* $moduleRoot
    if (Test-Path "$moduleRoot\node_modules") {
        Remove-Item -Recurse -Force "$moduleRoot\node_modules"
    }
    if (Test-Path "$moduleRoot\tests") {
        Remove-Item -Recurse -Force "$moduleRoot\tests"
    }
}

RunGoBuild "github.com/pulumi/pulumi"
RunGoBuild "github.com/pulumi/pulumi/sdk/nodejs/cmd/pulumi-language-nodejs"
RunGoBuild "github.com/pulumi/pulumi/sdk/python/cmd/pulumi-language-python"
RunGoBuild "github.com/pulumi/pulumi/sdk/dotnet/cmd/pulumi-language-dotnet"
RunGoBuild "github.com/pulumi/pulumi/sdk/go/pulumi-language-go"
CopyPackage "$Root\sdk\nodejs\bin" "pulumi"

Copy-Item "$Root\sdk\nodejs\dist\pulumi-resource-pulumi-nodejs.cmd" "$PublishDir\bin"
Copy-Item "$Root\sdk\python\dist\pulumi-resource-pulumi-python.cmd" "$PublishDir\bin"
Copy-Item "$Root\sdk\nodejs\dist\pulumi-analyzer-policy.cmd" "$PublishDir\bin"
Copy-Item "$Root\sdk\python\cmd\pulumi-language-python-exec" "$PublishDir\bin"

# By default, if the archive already exists, 7zip will just add files to it, so blow away the existing
# archive if it exists.
if (Test-Path $PublishFile) {
    Remove-Item -Force $PublishFile
}

7z a "$PublishFile" "$PublishDir\." | Out-Null

Remove-Item -Recurse -Force $PublishDir

New-Object PSObject -Property @{ArchivePath=$PublishFile;Targets=$PublishTargets}
