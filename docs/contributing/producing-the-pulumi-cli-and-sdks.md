# Producing the Pulumi CLI / SDKs

## Overview

`pulumi/pulumi` releases are centrally managed by the core team, so this
document is aimed at maintainers rather than external contributors. We
typically do a release every week: a minor release if there are new features,
else a patch release if just bugfixes. We also do releases in between as
needed.

The canonical step-by-step release procedure lives in the
[Releasing section of CONTRIBUTING.md](https://github.com/pulumi/pulumi/blob/master/CONTRIBUTING.md#releasing).
This document supplements it with the surrounding process: including language
provider releases, verifying the release, and retriggering post-release tasks.

## Language providers

Some language runtimes (YAML, Java, and others) live in their own repositories
and are bundled into Pulumi releases as binaries. The versions to bundle are
pinned in
[scripts/get-language-providers.sh](https://github.com/pulumi/pulumi/blob/master/scripts/get-language-providers.sh).

### Releasing YAML

This is managed by the team that owns the
[pulumi-yaml](https://github.com/pulumi/pulumi-yaml) repo, and can be done at
any point - minutes to days - prior to a release of the CLI. The release lead
for a CLI release will not typically need to perform these steps. See the
[Release section of pulumi-yaml's CONTRIBUTING.md](https://github.com/pulumi/pulumi-yaml/blob/main/CONTRIBUTING.md#release)
for the procedure (changie-driven: merging the changelog PR triggers the
release workflow).

### Releasing Java

See the
[Release Process section of pulumi-java's CONTRIBUTING.md](https://github.com/pulumi/pulumi-java/blob/main/CONTRIBUTING.md#release-process).

### Including a language provider release in the Pulumi release

Note the released version from the provider repo's releases page, then draft a
PR that:

* Updates the pinned version in `scripts/get-language-providers.sh`.
* Adds a changelog entry (`make changelog`) noting the provider release.

## Notifying the team

Shoot a note in an internal engineering channel when the release kicks off, and
follow up with a message when the new version is released.

## Releasing new CLI and SDK versions

Follow the
[Releasing section of CONTRIBUTING.md](https://github.com/pulumi/pulumi/blob/master/CONTRIBUTING.md#releasing):
freeze the draft release by bumping `sdk/.version` with
`./scripts/set-version.py <next-version>` (no `v` prefix), run the examples and
templates test suites for extra confidence, then publish the frozen draft
release.

Before publishing, ensure the draft release has all of the expected artifacts:

* `pulumi-` archives with the CLI for `darwin`, `linux`, and `windows`, each
  with `x64` (aka amd64) and `arm64` (aka aarch64) binaries.
* nodejs SDK: a `.tgz` file
* python SDK: a `.whl` file
* checksums: `B3SUMS`, `SHA512SUMS`, and a `pulumi-<version>-checksums.txt`

Proofread and edit the changelog in the GitHub Release if needed - these edits
will be reflected in the post-release PR. After the release succeeds,
`pulumi-bot` creates a post-release PR to update `CHANGELOG.md`, remove the
pending changelog entries, and update `go.mod` versions. Approve it and it will
auto-merge.

### Docs update

There will be an automatically created PR in the
[docs repo](https://github.com/pulumi/docs/pulls) called
`Regen docs pulumi@X.Y.Z`. Once it's green, approve and merge it to update the
[latest release version indicator](https://www.pulumi.com/latest-version)
among other things.

### Release verification

Once the docs PR has deployed (see the
[Build and deploy workflow](https://github.com/pulumi/docs/actions/workflows/build-and-deploy.yml?query=branch%3Amaster)),
running an older CLI should prompt that a new version of Pulumi is available.

## What the release does

Publishing the release triggers the
[release workflow](https://github.com/pulumi/pulumi/blob/master/.github/workflows/release.yml),
which:

* Publishes the SDKs (nodejs to npm, python to PyPI, go module tags) and the
  npm CLI wrapper package
* Uploads the release binaries to `get.pulumi.com`
* Creates the post-release PR
* Updates the Homebrew tap ([pulumi/homebrew-tap](https://github.com/pulumi/homebrew-tap))
* Dispatches post-release tasks to other repositories:
  * Update templates version (`pulumi/templates`)
  * Chocolatey update
  * Winget update
  * Package docs build
  * Homebrew bump
  * Docker containers (`pulumi/pulumi-docker-containers`)

The dispatch tasks run via [pulumictl](https://github.com/pulumi/pulumictl) as
command dispatch jobs within GitHub Actions. They all happen in parallel and,
as they are command dispatch events, they can be retriggered. To retrigger one,
set a bot token with release permissions (from the org secret store) as
`GITHUB_TOKEN` in your environment and run the appropriate command from the
`dispatch` job matrix in the release workflow, e.g.:

```sh
pulumictl dispatch -r pulumi/pulumi-docker-containers -c release-build 3.254.0
```
