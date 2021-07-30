"""Defines test subsets.

When removing or introducing new test subsets, make sure the
`test-subset` build matrix in these files:

- `.github/workflows/run-build-and-acceptance-tests.yml`
- `.github/workflows/master.yml`
- `.github/workflows/release.yml`

An implied subset `etc` will catch tests not matched by any
explicit subset listed here.

A note on the format of TEST_SUBSETS. The keys are test configuration
names, and the values are lists of either Go packages containing the
tests, or test suites names as passed to `run-testsuite.py`.

"""

TEST_SUBSETS = {
    'integration': [
        'github.com/pulumi/pulumi/tests/integration'
    ],
    'auto-and-lifecycletest': [
        'github.com/pulumi/pulumi/sdk/v3/go/auto',
        'github.com/pulumi/pulumi/pkg/v3/engine/lifeycletest'
    ],
    'native': [
        'dotnet-test',
        'istanbul',
        'istanbul-with-mocks',
        'python/lib/test',
        'python/lib/test/langhost/resource_thens',
        'python/lib/test_with_mocks'
    ]
}
