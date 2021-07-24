// Copyright 2016-2021, Pulumi Corporation

using Xunit;

namespace Pulumi.Tests.Resources
{
    public class DependencyProviderResourceTests
    {
        [Fact]
        public void GetPackage()
        {
            var res = new DependencyProviderResource("urn:pulumi:stack::project::pulumi:providers:aws::default_4_13_0");
            Assert.Equal("aws", res.Package);
        }
    }
}
