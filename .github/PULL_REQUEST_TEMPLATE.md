## Summary
<!-- What changed and why. Link to issue if applicable. -->
<!-- Remember: we squash-merge, so this description becomes the commit message. -->

## Test plan
<!-- How did you test this change? -->
- [ ] Added appropriate unit tests - For all changes
- [ ] Added a test in `pkg/engine/lifecycletest` - For all engine/protocol changes
- [ ] Added a conformance test in `pkg/testing/pulumi-test-language` - For language protocol changes
- [ ] Added a golden test in `pkg/backend/display` - For changes to the output renderers

## Validation
<!-- Commands you ran. Do not include output, but make sure that you've run these and checked the results. -->
- [ ] `make lint` — clean
- [ ] `make test_fast` — all pass
- [ ] `make tidy_fix` — clean
- [ ] `make format` — clean
- [ ] Relevant SDK tests pass (if SDK changes)
- [ ] `make check_proto` — clean (if proto changes)

## Changelog
<!-- Does this PR need a changelog entry? If so, did you run `make changelog`? -->
- [ ] Changelog entry added. If you do not believe this PR requires a changelog, ask a maintainer to apply
  the `impact/no-changelog-required` label.

## Risk
<!-- What could go wrong? What's the blast radius? Does this affect public API? -->

<!--

NOTE: maintainer time is a limited resource. Pull requests that do not follow this template can create
avoidable work and may be closed without review. Repeatedly ignoring these guidelines may result in
temporary or permanent restrictions to your ability to contribute to this project.

-->
