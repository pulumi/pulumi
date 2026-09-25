---
name: snapshot-integrity-failure
description: Triage a lifecycle fuzz (`TestFuzz`) snapshot integrity failure from CI. Replays the rapid seed, minimizes the generated repro into a deterministic lifecycle test, adds a fuzz exclusion rule, and files the tracking issue. Use when the `CI / Fuzz Test` job fails with "Encountered a snapshot integrity error", or when asked to address a snapshot integrity failure.
---

# Snapshot integrity failure triage

`pkg/engine/lifecycletest.TestFuzz` generates random snapshots, programs, and
plans and runs them through the engine. A failure means the engine produced a
snapshot that fails `Snapshot.VerifyIntegrity`. This is an engine bug. The
deliverable depends on how old the bug is:

1. A deterministic lifecycle test that reproduces the bug.
2. A bisect that names the commit that introduced it. This step is required.
3. A GitHub issue that names that commit.
4. If the commit is less than four weeks old: a proposal to revert it. Do not
   add an exclusion rule or a skip.
5. If the commit is older: an exclusion rule in
   `pkg/engine/lifecycletest/fuzzing/exclude.go` that keeps `TestFuzz` green,
   and the test from step 1 skipped with a `TODO[<issue url>]`.

Read `pkg/engine/lifecycletest/fuzzing/README.md` for how the fuzzer works.

## 1. Get the failure

Fetch the job log with the API. `gh run view --log` can 502 or claim the run
is still in progress.

```sh
gh api --allow-escape-sequences repos/pulumi/pulumi/actions/jobs/<job-id>/logs > job.log
```

Find these markers in the log:

- `Encountered a snapshot integrity error` and the `Messages:` line. The
  message names the resource and the invariant that failed.
- The pretty-printed `Snapshot`, `Program`, `Provider`, and `Plan` specs.
- `To reproduce, specify -run="TestFuzz" ... (or -rapid.seed=N)`. Take the seed.

The log also contains a `Full test case:` section. Do not use it. GitHub
masks every `{` and `}` as `***`, and the reconstruction is error-prone. The
local replay in the next step writes the same file unmasked.

## 2. Replay the seed locally

The seed is deterministic across machines. Run it from the `pkg` module:

```sh
cd pkg
PULUMI_LIFECYCLE_TEST_FUZZ=1 PULUMI_LIFECYCLE_TEST_FUZZING_REPRO_DIR=/tmp/fuzzrepro \
  go test -count=1 -tags all ./engine/lifecycletest/ -run '^TestFuzz$' -rapid.seed=<seed>
```

Put the package path before the `-run` and `-rapid.*` flags. `go test` stops
parsing flags at the first flag it does not know, and then it tests `.`.

The failure should occur after 0 tests. The output ends with one or more
`Reproduction test case was written to <path>` lines (rapid writes one file
per shrink attempt; they are equivalent). Each file holds two tests:
`TestReproSnapshot` (hard-coded starting snapshot) and `TestReproFramework`
(starting snapshot built by a setup update). Copy the file into
`pkg/engine/lifecycletest/` and run both. If the generated file does not
compile, fix it by hand and fix `fuzzing/reprogen.go` in the same PR.

If the seed does not reproduce, rebuild the fixture by hand from the
pretty-printed specs in the log. A race in step ordering reproduces only
sometimes. In that case loop the update 30 times from a fresh snapshot in the
deterministic test.

## 3. Read the journal

The failing run prints the step journal from `framework.go`:

- `test journal:` lists the steps in execution order. Each step appears twice
  (`0` = begin, `1` = success).
- `journal:` lists the snapshot mutations. `removeOld:N` removes resource N
  from the snapshot. `removePendingReplacementOld:N` marks it pending
  replacement and keeps it.

Find the first step after which the snapshot violates the invariant in the
error message. That step is the bug. Read the matching code in
`pkg/resource/deploy/step_generator.go` and `step_executor.go` and state the
mechanism in one paragraph before you write any code.

## 4. Minimize

Copy `TestReproSnapshot` into a scratch test and remove features one at a
time while the test still fails. Remove, in this order: targets, resources
that take no step, provider failure hooks, parents, `RetainOnDelete`,
`PendingReplacement`, extra providers. Then test the variants that decide the
scope of the exclusion rule. For example: custom resource instead of
component, direct replacement instead of a `DeletedWith` cascade, no targets.

The minimal test goes next to the existing tests for the operation
(`update_test.go`, `refresh_test.go`, `destroy_test.go`, ...). Use plain
names (`pkgA`, `prov`, `resA`, `comp`). Build the snapshot by hand and call
`require.NoError(t, snap.VerifyIntegrity())` on it. Model the program on the
user-facing trigger (a resource option, a provider diff result), not on the
fuzzer's internals. Run it to confirm it fails.

## 5. Bisect

Find the commit that introduced the bug before you decide what to do about
it. Use a throwaway worktree so the branch you are working on stays intact:

```sh
git worktree add --detach /tmp/bisect origin/master
(cd /tmp/bisect && mise trust)
```

Write the minimal test from step 4 as a standalone file with a stable name
(for example `TestBisectProbe`). Use `&b` for `*bool` options instead of
`new(true)`. The test framework API drifts, so the probe must try several
spellings of the same file, newest first, and use the first one that builds:

- `State` from `pkg/v3/resource` (since 2026-07-16) or from
  `sdk/v3/go/common/resource`.
- `plugin` from `pkg/v3/resource/plugin` (since 2026-06) or from
  `sdk/v3/go/common/resource/plugin`.
- `providers.NewReference` from `sdk/v3/go/common/providers` (since
  2025-11) or from `pkg/v3/resource/deploy/providers`.
- `deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)` or
  the older four-argument form without the two `nil`s before `loaders`.

Generate the variants with `sed` from the newest file rather than keeping
copies by hand.

Old commits pin other tool versions in `.mise.toml`, and `mise exec` then
tries to install them and hits the GitHub rate limit. Call the installed Go
binary directly, with `GOTOOLCHAIN=local` so it does not download another:

```sh
GO=$HOME/.local/share/mise/installs/go/<version>/bin/go
env -u GOROOT GOTOOLCHAIN=local CC=clang $GO test -count=1 -tags all ./engine/lifecycletest/ -run '^TestBisectProbe$'
```

Write a probe script that writes a variant into
`pkg/engine/lifecycletest/`, runs `go vet` on the package, and:

- exits 0 when the test passes,
- exits 1 when the test fails,
- exits 125 when no variant builds, so `git bisect run` skips the commit.

Always delete the copied file before the script exits.

First check the commit from four weeks ago:

```sh
git -C /tmp/bisect checkout --detach "$(git log --first-parent --format=%h -1 --before=<4 weeks ago> origin/master)"
./probe.sh
```

If the test passes there, the bug is recent. Bisect between that commit and
master. If it fails there, step back further (three months, six months, a
year) until it passes, then bisect from that point. Bisect with
`--first-parent` or the walk wanders into merged side histories:

```sh
cd /tmp/bisect
git bisect start --first-parent <bad> <good>
git bisect run ./probe.sh
git bisect reset
```

A race in step ordering needs a probe that loops the update 30 times.
Record the first bad commit, its date, and its PR number.

Stop once the probe fails at a checkpoint more than a year old. Record the
checkpoints you tested and the oldest one that fails; do not walk further.

Two commits changed what the test framework verifies rather than what the
engine does. If the bisect lands on one of them, the engine behavior is
older than the commit, and the bisect only tells you when the framework
started to detect it:

- `035a502d86` "Verify partial journals in engine tests" (#15018,
  2024-01-05): every intermediate snapshot is verified, not only the final
  one.
- `d28fa18eec` "Test SnapshotManager and Journal in engine tests" (#15871,
  2024-04-11): the real snapshot manager runs alongside the journal.

## 6. Decide: revert or exclude

If the first bad commit is less than four weeks old, file the issue (step 8)
and propose a revert of that commit in the issue and in the chat. Do not add
an exclusion rule and do not skip the test. Stop here.

If it is older, continue with the exclusion rule and the skipped test. In the
issue, name the first bad commit, or the checkpoints you tested if you
stopped at a year.

## 7. Exclusion rule

Add `Exclude<Scenario><Operation>` to `exclude.go` and register it in
`DefaultExclusionRules` behind a `// TODO[pulumi/pulumi#N]` comment. The rule
takes the four specs and returns `true` to reject the fixture. Key it on the
spec features the minimization showed to be necessary and nothing else. Check
`plan.Operation` first. Check the snapshot and the program registrations
separately: a link that only exists in one of them still triggers the engine
path. Write the mechanism in the doc comment.

Verify the rule with the original seed. The replay must now pass.

```sh
cd pkg
PULUMI_LIFECYCLE_TEST_FUZZ=1 go test -count=1 -tags all ./engine/lifecycletest/ \
  -run '^TestFuzz$' -rapid.seed=<seed> -rapid.checks=2000
```

Then run a fresh batch: `make test_lifecycle_fuzz LIFECYCLE_TEST_FUZZ_CHECKS=2000`.

## 8. File the issue, then link it

Use the `file-issue` skill. Describe the user-visible scenario, the error
line, and the step order from the journal. Do not describe the fix. Mention
the CI job URL, the seed, and the first bad commit from the bisect with its
PR number. For a recent commit, say that a revert is proposed.

For an old commit, then:

- Put the issue number in the `TODO[pulumi/pulumi#N]` above the rule.
- Add the skip at the top of the deterministic test:

  ```go
  // TODO[https://github.com/pulumi/pulumi/issues/N]: Fix the underlying issue and re-enable this test.
  t.Skip("Skipping: <one-line symptom>")
  ```

## 9. Finish

- Delete the scratch tests, the copied generated file, and the bisect
  worktree (`git worktree remove /tmp/bisect`).
- `cd pkg && go test -count=1 -tags all ./engine/lifecycletest/ -run '<new test>' -v`
  must report `SKIP`.
- `make lint_golang`.
- No changelog entry: the change is test-only.

Prior examples: `ExcludeComponentWithProviderRefreshProgram` with
`TestRefreshProgramUpdateReplacedComponentProvider` (#24680), and
`ExcludeTargetedUpdateRefreshWithDeletedParent` with
`TestTargetedUpdateRefreshWithDeletedParent` (#22923).
