Set-StrictMode -Version 2.0
$ErrorActionPreference="Stop"

git update-index -q --refresh
git diff-files --quiet | Out-Null

$dirty=($LASTEXITCODE -ne 0)

try { 
  git describe --tags --exact-match >$null 2>$null
  # If we get here the above did not throw, so we can use the exact tag
  if ($dirty) {
      Write-Output "$(git describe --tags --exact-match)"
  } else {
      Write-Output "$(git describe --tags --exact-match)+dirty"
  }
} catch {
    # Otherwise, take the existing tag, increment the patch version and append the timestamp of the commit and hash
    $tag=""

    try {
        git describe --tags --abbrev=0 >$null 2>$null
        $tag="$(git describe --tags --abbrev=0)"
    } catch {
        $tag="v0.0.0"
    }

    # Remove any pre-release tag
    if ($tag.LastIndexOf("-") -ne -1) {
        $tag=$tag.Substring(0, $tag.LastIndexOf("-"))
    }

    $tagParts = $tag.Split('.');
    $major = $tagParts[0];
    $minor = $tagParts[1];
    $patch = $tagParts[2];

    $patch = ([int]$patch + 1);
    if ($dirty) {
        Write-Output "$major.$minor.$patch-dev.$(git show -s --format='%ct+g%h').dirty"
    } else {
        Write-Output "$major.$minor.$patch-dev.$(git show -s --format='%ct+g%h')"
    }
}

