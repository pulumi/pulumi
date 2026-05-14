# The Cloud-Ready CLI — Merged PRs

Merged pull requests contributing to [epic #22959 — The Cloud-Ready CLI](https://github.com/pulumi/pulumi/issues/22959), recursively. Listed oldest to newest. PRs are grouped by category; each entry notes the closed sub-issue where known.

## Foundation — `pulumi api` dispatcher and command scaffolding

- [#22769 — cli/cloud: add `pulumi cloud api list`](https://github.com/pulumi/pulumi/pull/22769) — @fallimic (2026-05-01)
- [#22770 — cli/cloud: add `pulumi cloud api describe`](https://github.com/pulumi/pulumi/pull/22770) — @fallimic (2026-05-01)
- [#22771 — cli/cloud: add `pulumi cloud api` dispatcher](https://github.com/pulumi/pulumi/pull/22771) — @fallimic (2026-05-01)
- [#22772 — cli/cloud: add `--paginate` to `pulumi cloud api`](https://github.com/pulumi/pulumi/pull/22772) — @fallimic (2026-05-01)
- [#22874 — cli/cloud: auto-fit `cloud api list` table to terminal width](https://github.com/pulumi/pulumi/pull/22874) — @fallimic (2026-05-07)
- [#22881 — cli/cloud: nudge users to `describe` before calling `pulumi cloud api`](https://github.com/pulumi/pulumi/pull/22881) — @kramhuber (2026-05-11)
- [#22970 — cli/cloud: rename `pulumi cloud api` to `pulumi api`](https://github.com/pulumi/pulumi/pull/22970) — @fallimic (2026-05-12)
- [#23071 — cli: scaffold hidden cobra commands for the Cloud-Ready CLI](https://github.com/pulumi/pulumi/pull/23071) — @iwahbe (2026-05-12)
- [#23072 — cli/cloud: rename `--format` flag to `--output` on `pulumi api`](https://github.com/pulumi/pulumi/pull/23072) — @i-am-tom (2026-05-12)
- [#23125 — \[cli/cloud\] Fix `pulumi api` help examples to use real operation IDs](https://github.com/pulumi/pulumi/pull/23125) — @fallimic (2026-05-13)

## `pulumi template` — closes epic [#22960](https://github.com/pulumi/pulumi/issues/22960)

- [#23074 — cli/cloud: add `pulumi template list`](https://github.com/pulumi/pulumi/pull/23074) — @i-am-tom (2026-05-13)

## `pulumi insights resource` — closes epic [#22973](https://github.com/pulumi/pulumi/issues/22973)

- [#23077 — cli/cloud: add `pulumi insights resource get`](https://github.com/pulumi/pulumi/pull/23077) — @i-am-tom (2026-05-13)
- [#23087 — cli/cloud: add `pulumi insights resource search`](https://github.com/pulumi/pulumi/pull/23087) — @i-am-tom (2026-05-13)

## `pulumi deployment`

- [#23114 — cli/cloud: add `pulumi deployment list`](https://github.com/pulumi/pulumi/pull/23114) — @i-am-tom (2026-05-13) — closes [#22988](https://github.com/pulumi/pulumi/issues/22988)

## `pulumi stack`

- [#23082 — implement `pulumi stack webhook list`](https://github.com/pulumi/pulumi/pull/23082) — @tgummerer (2026-05-13) — closes [#23062](https://github.com/pulumi/pulumi/issues/23062)
- [#23088 — implement `pulumi stack webhook get`](https://github.com/pulumi/pulumi/pull/23088) — @tgummerer (2026-05-13) — closes [#23061](https://github.com/pulumi/pulumi/issues/23061)
- [#23089 — add `pulumi stack webhook ping`](https://github.com/pulumi/pulumi/pull/23089) — @tgummerer (2026-05-14) — closes [#23057](https://github.com/pulumi/pulumi/issues/23057)
- [#23101 — implement `pulumi stack webhook new`](https://github.com/pulumi/pulumi/pull/23101) — @tgummerer (2026-05-14) — closes [#23060](https://github.com/pulumi/pulumi/issues/23060)
- [#23106 — cli/cloud: implement `pulumi stack get`](https://github.com/pulumi/pulumi/pull/23106) — @iwahbe (2026-05-14) — closes [#23065](https://github.com/pulumi/pulumi/issues/23065)

## Notes

The following Cloud-Ready CLI sub-epics are marked `CLOSED` on GitHub but have no associated merged PR discoverable through the `closedByPullRequestsReferences` GraphQL link or via title/body search. They appear to have been closed administratively (e.g. as duplicates or out-of-scope):

- [#22999 — `pulumi org billing`](https://github.com/pulumi/pulumi/issues/22999)
- [#23020 — `pulumi org invite`](https://github.com/pulumi/pulumi/issues/23020)
- [#23021 — `pulumi org invite get`](https://github.com/pulumi/pulumi/issues/23021)
