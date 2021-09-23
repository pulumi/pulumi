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

The special `etc` test subset catches all unlisted tests. This subset
will always run on every PR. Other tests subsets may be skipped on PR
verification for platforms such as Mac OS where there is currently a
shortage of runners; these tests will still run on `master` and
`release` verifications.

"""

TEST_SUBSETS = {
    'integration': [
        'github.com/pulumi/pulumi/tests/integration',
    ],

    'integration-and-codegen': [
        'github.com/pulumi/pulumi/tests/integration/aliases',
        'github.com/pulumi/pulumi/tests/integration/custom_timeouts',
        'github.com/pulumi/pulumi/tests/integration/delete_before_create',
        'github.com/pulumi/pulumi/tests/integration/dependency_steps',
        'github.com/pulumi/pulumi/tests/integration/double_pending_delete',
        'github.com/pulumi/pulumi/tests/integration/duplicate_urns',
        'github.com/pulumi/pulumi/tests/integration/partial_state',
        'github.com/pulumi/pulumi/tests/integration/policy',
        'github.com/pulumi/pulumi/tests/integration/protect_resources',
        'github.com/pulumi/pulumi/tests/integration/query',
        'github.com/pulumi/pulumi/tests/integration/read/import_acquire',
        'github.com/pulumi/pulumi/tests/integration/read/read_dbr',
        'github.com/pulumi/pulumi/tests/integration/read/read_relinquish',
        'github.com/pulumi/pulumi/tests/integration/read/read_replace',
        'github.com/pulumi/pulumi/tests/integration/recreate_resource_check',
        'github.com/pulumi/pulumi/tests/integration/steps',
        'github.com/pulumi/pulumi/tests/integration/targets',
        'github.com/pulumi/pulumi/tests/integration/transformations',
        'github.com/pulumi/pulumi/tests/integration/types',
        'github.com/pulumi/pulumi/pkg/v3/codegen',
        'github.com/pulumi/pulumi/pkg/v3/codegen/docs',
        'github.com/pulumi/pulumi/pkg/v3/codegen/dotnet',
        'github.com/pulumi/pulumi/pkg/v3/codegen/go',
        'github.com/pulumi/pulumi/pkg/v3/codegen/hcl2',
        'github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model',
        'github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax',
        'github.com/pulumi/pulumi/pkg/v3/codegen/importer',
        'github.com/pulumi/pulumi/pkg/v3/codegen/nodejs',
        'github.com/pulumi/pulumi/pkg/v3/codegen/python',
        'github.com/pulumi/pulumi/pkg/v3/codegen/schema',
    ],

    'auto': [
        # Primary Go-driven Auto API tests
        'github.com/pulumi/pulumi/sdk/v3/go/auto',

        # Auto API tests driven by dotnet
        'auto-dotnet',

        # Auto API tests driven by npm
        'auto-nodejs',

        # Auto API tests driven by pytest
        'auto-python',
    ]
}
