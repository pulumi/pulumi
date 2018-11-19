param (
  $VersionTag, 
  $PulumiRef
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = "Stop"

$S3ProdBucketRoot="s3://get.pulumi.com/releases/"
$S3EngBucketRoot="s3://eng.pulumi.com/releases/"
$S3PublishFolderSdk="${S3ProdBucketRoot}sdk/"

function New-TemporaryDirectory {
    New-Item -ItemType Directory -Path (Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName()))
}

function New-TemporaryFile {
    New-Item -ItemType File -Path (Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName()))
}

function Download-Release ($repoName, $repoCommit, [ValidateSet("zip", "tgz")]$ext) {
    Write-Host "downloading $repoName@$repoCommit"

    $file="${repoCommit}.${ext}"
    $s3File="${S3EngBucketRoot}${repoName}/windows/amd64/${file}"
    
    aws s3 cp --only-show-errors "$s3File" ".\$file"

    switch($ext) {
        "zip" { 7z x ${file} }
        "tgz" { cmd /C "7z x ${file} -tgzip -so | 7z x -ttar -si" }
        default { Write-Error "Unknown extention type $ext" }
    }

    Remove-Item -Force "$file"
}

if (!$VersionTag) { $VersionTag=Get-Date -UFormat '%Y%m%d_%H%M%S' }
if (!$PulumiRef) { $PulumiRef="master" }

$SdkFileName="pulumi-$($VersionTag -replace '\+.', '')-windows-x64.zip"

$PulumiFolder=(Join-Path (New-TemporaryDirectory) "Pulumi")

New-Item -ItemType Directory -Path $PulumiFolder | Out-Null

Push-Location "$PulumiFolder" | Out-Null

Write-Host "pulumi:       $PulumiRef"
Write-Host ""

Download-Release "pulumi" $PulumiRef "zip"

Remove-Item -Recurse -Force -Path "$PulumiFolder\node_modules"

$SdkPackagePath=(Join-Path ([System.IO.Path]::GetTempPath()) $SdkFileName)

if (Test-Path $SdkPackagePath) {
    Remove-Item -Force -Path $SdkPackagePath
}

7z a -tzip "$SdkPackagePath" "$(Join-Path (Split-Path -Parent $PulumiFolder) '.')"

Write-Host "uploading SDK to ${S3PublishFolderSdk}${SdkFileName}"

$AWSCreds=((aws sts assume-role `
               --role-arn "arn:aws:iam::058607598222:role/UploadPulumiReleases" `
               --role-session-name "upload-sdk" `
               --external-id "upload-pulumi-release") | ConvertFrom-Json)

$env:AWS_ACCESS_KEY_ID=$AWSCreds.Credentials.AccessKeyId
$env:AWS_SECRET_ACCESS_KEY=$AWSCreds.Credentials.SecretAccessKey
$env:AWS_SECURITY_TOKEN=$AWSCreds.Credentials.SessionToken

aws s3 cp --acl public-read --only-show-errors "$SdkPackagePath" "${S3PublishFolderSdk}${SdkFileName}"

Pop-Location | Out-Null

Remove-Item -Path $SdkPackagePath
Remove-Item -Path (Split-Path -Parent $PulumiFolder) -Force -Recurse

Write-Host "done"
