// Copyright 2016-2021, Pulumi Corporation

using Pulumi;

namespace Pulumi.Mypkg
{
    [ResourceType("mypkg::mockdep", null)]
    public sealed class MockDep : CustomResource
    {
        [Output("mockDepOutput")]
        public Output<string> MockDepOutput { get; private set; } = null!;

        public MockDep(string name, MockDepArgs? args = null, CustomResourceOptions? options = null)
            : base("mypkg::mockdep", name, args ?? new MockDepArgs(), options)
            {
            }
    }

    public sealed class MockDepArgs : ResourceArgs
    {
    }
}
