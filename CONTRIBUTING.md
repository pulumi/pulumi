# Contributing to Pulumi

First, thanks for contributing to Pulumi and helping make it better. We appreciate the help!
This repository is one of many across the Pulumi ecosystem and we welcome contributions to them all.

## Code of Conduct

Please make sure to read and observe our [Contributor Code of Conduct](./CODE-OF-CONDUCT.md).

## Communications

You are welcome to join the [Pulumi Community Slack](https://slack.pulumi.com/) for questions and a community of like-minded folks.
We discuss features and file bugs on GitHub via [Issues](https://github.com/pulumi/pulumi/issues) as well as [Discussions](https://github.com/pulumi/pulumi/discussions).
You can read about our [public roadmap](https://github.com/orgs/pulumi/projects/44) on the [Pulumi blog](https://www.pulumi.com/blog/relaunching-pulumis-public-roadmap/).

### Issues

Feel free to pick up any existing issue that looks interesting to you or fix a bug you stumble across while using Pulumi. No matter the size, we welcome all improvements.

### Feature Work

For larger features, we'd appreciate it if you open a [new issue](https://github.com/pulumi/pulumi/issues/new) before investing a lot of time so we can discuss the feature together.
Please also be sure to browse [current issues](https://github.com/pulumi/pulumi/issues) to make sure your issue is unique, to lighten the triage burden on our maintainers.
Finally, please limit your pull requests to contain only one feature at a time. Separating feature work into individual pull requests helps speed up code review and reduces the barrier to merge.

## Developing

### Setting up your Pulumi development environment

Please see the [developer docs](https://pulumi-developer-docs.readthedocs.io/latest/docs/contributing/development.html) for instructions.

## Submitting a Pull Request

For contributors we use the [standard fork based workflow](https://gist.github.com/Chaser324/ce0505fbed06b947d962): Fork this repository, create a topic branch, and when ready, open a pull request from your fork.

Before you open a pull request, make sure all lint checks pass:

```bash
$ make lint
```

If you see formatting failures, fix them by running [gofumpt](https://github.com/mvdan/gofumpt) on your code:

```bash
$ gofumpt -w path/to/file.go
# or
$ gofumpt -w path/to/dir
```

We require a changelog entry for all PR that aren't labeled `impact/no-changelog-required`. To generate a new changelog entry, run…

```bash
$ make changelog
````
…and follow the prompts on screen.

### Changelog messages

Changelog notes are written in the active imperative form.  They should not end with a period.  The simple rule is to pretend the message starts with "This change will ..."

Good examples for changelog entries are:
- Exit immediately from state edit when no change was made
- Fix root and program paths to always be absolute

Here's some examples of what we're trying to avoid:
- Fixes a bug
- Adds a feature
- Feature now does something

### Downloading Pulumi from contributed pull requests

Artifacts built during pull request workflows can be downloaded by running the following command (note that the artifacts expire 7 days after CI has been run):

```sh
curl -fsSL https://get.pulumi.com | sh -s -- --version pr#<number>
```

### Pulumi employees

Pulumi employees have write access to Pulumi repositories and should push directly to branches rather than forking the repository. Tests can run directly without approval for PRs based on branches rather than forks.

Please ensure that you nest your branches under a unique identifier such as your name (e.g. `refs/heads/pulumipus/cool_feature`).

## Understanding Pulumi

The Pulumi system is robust and offers many features that might seem overwhelming at first. To assist you in getting
started, our team has put together the [Pulumi Developer
Documentation](https://pulumi-developer-docs.readthedocs.io/latest/docs/README.html). This resource provides valuable
insights into how the system is structured and can be incredibly helpful for both new contributors and maintainers
alike. We encourage you to explore it and reach out if you have any questions!

## Getting Help

We're sure there are rough edges and we appreciate you helping out. If you want to talk with other folks in the Pulumi community (including members of the Pulumi team) come hang out in the `#contribute` channel on the [Pulumi Community Slack](https://slack.pulumi.com/).

## Releasing

Whenever a new PR is merged in this repository, the latest draft release on the [GitHub Releases page](https://github.com/pulumi/pulumi/releases) is updated with the latest binaries.  To release one of those draft releases a few steps are necessary:

If `sdk/.version` is the version we want to release, we need to "freeze" that draft release.  To do that update the version in `pulumi/pulumi` using `scripts/set-version.py <next-patch-version>`.  This stops the draft release for the current version from being updated, and thus it is ready to be released.

If `sdk/.version` is not the version we want to release yet, usually in the case of a minor release, bump the version to the right version first, and merge that first (always using `scripts/set-version.py`).  Once that's merged the current release can be frozen as above.

For these version bump PRs it's useful for reviewers if the expected changelog is included.  This can be generated using `GITHUB_REPOSITORY=pulumi/pulumi go run github.com/pulumi/go-change@v0.1.3 render`, at the root of the repository.

The next step, to gain some additional confidence in the release is to run the [Test examples](https://github.com/pulumi/examples/actions/workflows/test-examples.yml), and [Test templates](https://github.com/pulumi/templates/actions/workflows/test-templates.yml) test suites.  These run the tests in the `pulumi/examples` and `pulumi/templates` repositories using the latest `pulumi/pulumi` dev version, thus including all the latest changes.

Finally to create the release, navigate to the [GitHub Releases page](https://github.com/pulumi/pulumi/releases) and edit the release of the version we froze just before.  Untick "Set as a pre-release", and tick both "Set as the latest release" and "Create a discussion for this release" in the "Releases" category at the bottom of the page, before clicking "Publish release".

> [!CAUTION]
> Double-check the version number of the release. The most recent release in [the releases list](https://github.com/pulumi/pulumi/releases) tracks changes to `master`, and might not be the one you want to release if PRs have been merged since the freeze. For this reason, the version you want to release may be the second one in the list. The version should be the one that was in `sdk/.version` *before* the freeze PR was merged.

Finally `pulumi-bot` will create another PR to update with `go.mod` updates and changelog cleanups.  This PR needs to be approved, and will then auto-merge.
