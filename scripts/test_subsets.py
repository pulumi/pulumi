"""Defines test subsets.

When removing or introducing new test subsets, make sure the
`test-subset` build matrix in `run-build-and-acceptance-tests.yml`
matches. An implied subset `etc` will catch tests not matched by any
explicit subset listed here.

A note on the format of TEST_SUBSETS. The keys are test configuration
names, and the values are lists of either Go packages containing the
tests, or test suites names as passed to `run-testsuite.py`.

"""

from collections import namedtuple


TestSubset = namedtuple(
    'TestSubset',
    ['go_packages',
     'go_tags',
     'test_suites'],
    defaults={'go_packages': [],
              'go_tags': [],
              'test_suites': []})


TEST_SUBSETS = {
    'integration-python': TestSubset(
        go_packages=['github.com/pulumi/pulumi/tests/integration'],
        go_tags=['python'],
    ),
    'integration-nodejs': TestSubset(
        go_packages=['github.com/pulumi/pulumi/tests/integration'],
        go_tags=['nodejs'],
    ),
    'integration-go': TestSubset(
        go_packages=['github.com/pulumi/pulumi/tests/integration'],
        go_tags=['go'],
    ),
    'integration-dotnet': TestSubset(
        go_packages=['github.com/pulumi/pulumi/tests/integration'],
        go_tags=['dotnet'],
    ),
    'auto-and-lifecycletest': TestSubset(
        go_packages=[
            'github.com/pulumi/pulumi/sdk/v3/go/auto',
            'github.com/pulumi/pulumi/pkg/v3/engine/lifeycletest'
        ]
    ),
    'native': TestSubset(test_suites=[
        'dotnet-test',
        'istanbul',
        'istanbul-with-mocks',
        'python/lib/test',
        'python/lib/test/langhost/resource_thens',
        'python/lib/test_with_mocks'
    ])
}
