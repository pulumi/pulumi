# The Cloud-Ready CLI — Unassigned Work

Bill of work derived from epic [#22959 — The Cloud-Ready CLI](https://github.com/pulumi/pulumi/issues/22959).

This document lists every **OPEN** issue under that epic (recursively) that currently has **no assignee**, with explicit scope decisions applied:

- The entire `pulumi env` subtree ([#23022](https://github.com/pulumi/pulumi/issues/23022) and descendants) is **out of scope** for this PR.
- [#22994 `pulumi policy compliance list`](https://github.com/pulumi/pulumi/issues/22994) is **skipped for now**.

Issues are grouped under their parent epic for context. An entry marked _(epic)_ has its own sub-issues; entries without that marker are leaf work items.

## `pulumi deployment` — [#22982](https://github.com/pulumi/pulumi/issues/22982) _(epic, assigned to @i-am-tom)_

- [#22983 — `pulumi deployment settings edit`](https://github.com/pulumi/pulumi/issues/22983)
- [#22984 — `pulumi deployment settings get`](https://github.com/pulumi/pulumi/issues/22984)
- [#22986 — `pulumi deployment log`](https://github.com/pulumi/pulumi/issues/22986)
- [#22987 — `pulumi deployment get`](https://github.com/pulumi/pulumi/issues/22987)

## `pulumi policy` — [#22989](https://github.com/pulumi/pulumi/issues/22989) _(epic, unassigned)_

- [#22990 — `pulumi policy group remove`](https://github.com/pulumi/pulumi/issues/22990)
- [#22991 — `pulumi policy group edit`](https://github.com/pulumi/pulumi/issues/22991)
- [#22992 — `pulumi policy group get`](https://github.com/pulumi/pulumi/issues/22992)
- [#22993 — `pulumi policy group new`](https://github.com/pulumi/pulumi/issues/22993)
- [#22995 — `pulumi policy issue list`](https://github.com/pulumi/pulumi/issues/22995)
- [#22996 — `pulumi policy issue get`](https://github.com/pulumi/pulumi/issues/22996)

## `pulumi org` — [#22998](https://github.com/pulumi/pulumi/issues/22998) _(epic, unassigned)_

- [#23000 — `pulumi org audit-log export`](https://github.com/pulumi/pulumi/issues/23000)
- [#23001 — `pulumi org audit-log list`](https://github.com/pulumi/pulumi/issues/23001)

### `pulumi org member` — [#23009](https://github.com/pulumi/pulumi/issues/23009) _(epic, unassigned)_

- [#23010 — `pulumi org member remove`](https://github.com/pulumi/pulumi/issues/23010)
- [#23011 — `pulumi org member get`](https://github.com/pulumi/pulumi/issues/23011)
- [#23012 — `pulumi org member edit`](https://github.com/pulumi/pulumi/issues/23012)

> Note: [#22964 `pulumi org webhook`](https://github.com/pulumi/pulumi/issues/22964) is an unassigned epic, but every leaf sub-issue (#22965, #22966, #22967, #22968, #22969, #22997) is assigned to @tgummerer, so no additional work is needed here.

## `pulumi stack` — [#23044](https://github.com/pulumi/pulumi/issues/23044) _(epic, unassigned)_

- [#23066 — `pulumi stack new`](https://github.com/pulumi/pulumi/issues/23066)

> Note: the other `pulumi stack` sub-epics — [#23050 `stack schedule`](https://github.com/pulumi/pulumi/issues/23050) (@julienp), [#23053 `stack drift`](https://github.com/pulumi/pulumi/issues/23053) (@tgummerer), [#23063 `stack webhook`](https://github.com/pulumi/pulumi/issues/23063) (@tgummerer) — have all of their leaf sub-issues assigned, so no additional work is needed under them.

## Out of scope

- **`pulumi env` subtree** — We won't be working on Pulumi env. Covers [#23022](https://github.com/pulumi/pulumi/issues/23022) (assigned to @tehsis, @borisschlosser) and all descendants ([#23030 env webhook](https://github.com/pulumi/pulumi/issues/23030), [#23036 env schedule](https://github.com/pulumi/pulumi/issues/23036), and leaves #23023–#23029, #23031–#23035, #23041, #23042, #23043).
- **[#22994 — `pulumi policy compliance list`](https://github.com/pulumi/pulumi/issues/22994)** — skipped for now (the endpoint is named `GetPolicyComplianceResults` but is a POST; revisit once shape settles).

## Sanity-check carry-overs

Notes from the per-issue sanity check that still need attention before/during implementation:

- **[#23011 `pulumi org member get`](https://github.com/pulumi/pulumi/issues/23011)** — issue body describes `ListOrganizationMembers` (a list), but the title says `get` (single member). Either rename the command to `list` or repoint the body at the singular-member GET before implementing.
- **[#22983 `pulumi deployment settings edit`](https://github.com/pulumi/pulumi/issues/22983)** — the docs page has both PATCH (merge) and PUT (replace); `edit` should map to PATCH.

## Summary

- **16** unassigned leaf work items.
- **4** unassigned grouping epics: [#22989](https://github.com/pulumi/pulumi/issues/22989), [#22998](https://github.com/pulumi/pulumi/issues/22998), [#23009](https://github.com/pulumi/pulumi/issues/23009), [#23044](https://github.com/pulumi/pulumi/issues/23044). Closing the leaf items will close these out.

### Quick-reference list (issue numbers only)

```
22983 22984 22986 22987
22990 22991 22992 22993 22995 22996
23000 23001
23010 23011 23012
23066
```
