TEST_SUBSETS = {
    'integration': [
        'github.com/pulumi/pulumi/tests/integration',
        'github.com/pulumi/pulumi/tests/integration/aliases',
        'github.com/pulumi/pulumi/tests/integration/transformations'
    ],
    'lifecycletest': ['github.com/pulumi/pulumi/pkg/v3/engine/lifeycletest'],
    'auto': ['github.com/pulumi/pulumi/sdk/v3/go/auto'],
    'native': [
        'dotnet-test',
        'istanbul',
        'istanbul-with-mocks',
        'python/lib/test',
        'python/lib/test/langhost/resource_thens',
        'python/lib/test_with_mocks'
    ]
}
