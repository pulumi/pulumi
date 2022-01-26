// Copyright 2016-2022, Pulumi Corporation

using Xunit;

namespace Pulumi.Tests.Resources
{
    public class MergeResourceOptionsTests
    {
        [Fact]
        public void MergeCustom()
        {
            var prov = new DependencyProviderResource("urn:pulumi:stack::project::pulumi:providers:aws::default_4_13_0");
            var o1 = new CustomResourceOptions {
                Provider = prov,
            };
            var o2 = new CustomResourceOptions {
                Protect = true,
            };
            var result = CustomResourceOptions.Merge(o1, o2);
            Assert.Equal("aws", result.Provider!.Package);
            Assert.True(result.Protect);
        }
    }
}
