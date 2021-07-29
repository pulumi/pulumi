"""Defines test subsets.

When removing or introducing new test subsets, make sure the
`test-subset` build matrix in `run-build-and-acceptance-tests.yml`
matches. An implied subset `etc` will catch tests not matched by any
explicit subset listed here.

A note on the format of TEST_SUBSETS. The keys are test configuration
names, and the values are lists of either Go packages containing the
tests, or test suites names as passed to `run-testsuite.py`.

"""

TEST_SUBSETS = {
    'integration': [
        "github.com/pulumi/pulumi/tests/integration/aliases",
        "github.com/pulumi/pulumi/tests/integration/custom_timeouts",
        "github.com/pulumi/pulumi/tests/integration/delete_before_create",
        "github.com/pulumi/pulumi/tests/integration/dependency_steps",
        "github.com/pulumi/pulumi/tests/integration/double_pending_delete",
        "github.com/pulumi/pulumi/tests/integration/duplicate_urns",
        "github.com/pulumi/pulumi/tests/integration/partial_state",
        "github.com/pulumi/pulumi/tests/integration/policy",
        "github.com/pulumi/pulumi/tests/integration/protect_resources",
        "github.com/pulumi/pulumi/tests/integration/query",
        "github.com/pulumi/pulumi/tests/integration/read/import_acquire",
        "github.com/pulumi/pulumi/tests/integration/read/read_dbr",
        "github.com/pulumi/pulumi/tests/integration/read/read_relinquish",
        "github.com/pulumi/pulumi/tests/integration/read/read_replace",
        "github.com/pulumi/pulumi/tests/integration/recreate_resource_check",
        "github.com/pulumi/pulumi/tests/integration/steps",
        "github.com/pulumi/pulumi/tests/integration/targets",
        "github.com/pulumi/pulumi/tests/integration/transformations",
        "github.com/pulumi/pulumi/tests/integration/types",
        'github.com/pulumi/pulumi/tests/integration',
    ]
}

# The previous config used 4 runners and while it finished CI faster,
# the project started hitting throughput problems on not having enough
# Mac OS runners available:
#
# TEST_SUBSETS = {
#     'integration': [
#         'github.com/pulumi/pulumi/tests/integration'
#     ],
#     'auto-and-lifecycletest': [
#         'github.com/pulumi/pulumi/sdk/v3/go/auto',
#         'github.com/pulumi/pulumi/pkg/v3/engine/lifeycletest'
#     ],
#     'native': [
#         'dotnet-test',
#         'istanbul',
#         'istanbul-with-mocks',
#         'python/lib/test',
#         'python/lib/test/langhost/resource_thens',
#         'python/lib/test_with_mocks'
#     ]
# }
