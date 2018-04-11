Set-StrictMode -Version 2.0
$ErrorActionPreference="Stop"

git update-index -q --refresh
git diff-files --quiet | Out-Null

$dirtyTag=""
if ($LASTEXITCODE -ne 0) {
    $dirtyTag = "-dirty"
}

try { 
  git describe --tags --exact-match >$null 2>$null
  # If we get here the above did not throw, so we can use the exact tag
  Write-Output "$(git describe --tags --exact-match)$dirtyTag"
} catch {
  # Otherwise, append the timestamp of the commit and the hash
  Write-Output "$(git describe --tags --abbrev=0)-$(git show -s --format='%ct-g%h')$dirtyTag"
}

