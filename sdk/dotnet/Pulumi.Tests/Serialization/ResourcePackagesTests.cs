// Copyright 2016-2020, Pulumi Corporation

using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class ResourcePackagesTests
    {
        [Fact]
        public void UnknownNotFound()
        {
            if (ResourcePackages.TryGetResourcePackage("unknown", null, out _))
            {
                Assert.True(false, "Unknown package found");
            }
            if (ResourcePackages.TryGetResourcePackage("unknown", "0.0.1", out _))
            {
                Assert.True(false, "Unknown package found");
            }
        }
        
        [Fact]
        public void BlankReturnsHighestVersion()
        {
            if (ResourcePackages.TryGetResourcePackage("test", null, out var package))
            {
                Assert.Equal("test", package.Name);
                Assert.Equal("2.2.0", package.Version);
            }
            else
            {
                Assert.True(false, "Test package not found");
            }
        }
        
        [Fact]
        public void MajorVersionRespected()
        {
            if (ResourcePackages.TryGetResourcePackage("test", "1.0.0", out var package))
            {
                Assert.Equal("test", package.Name);
                Assert.Equal("1.0.2", package.Version);
            }
            else
            {
                Assert.True(false, "Test package not found");
            }
        }
        
        [Fact]
        public void WildcardSelectedIfOthersDontMatch()
        {
            if (ResourcePackages.TryGetResourcePackage("test", "3.0.0", out var package))
            {
                Assert.Equal("test", package.Name);
                Assert.Null(package.Version);
            }
            else
            {
                Assert.True(false, "Test package not found");
            }
        }

        private abstract class BaseTestPackage : IResourcePackage
        {
            public abstract string Name { get; }
        
            public abstract string? Version { get; }
    
            public ProviderResource ConstructProvider(string name, string type, string urn)
            {
                throw new System.NotImplementedException();
            }

            public Resource Construct(string name, string type, string urn)
            {
                throw new System.NotImplementedException();
            }
        }
        
        private class Version101TestPackage : BaseTestPackage
        {
            public override string Name => "test";
            public override string? Version => "1.0.1-alpha1";
        }
        
        private class Version102TestPackage : BaseTestPackage
        {
            public override string Name => "test";
            public override string? Version => "1.0.2";
        }
        
        private class Version220TestPackage : BaseTestPackage
        {
            public override string Name => "test";
            public override string? Version => "2.2.0";
        }
        
        private class WildcardTestPackage : BaseTestPackage
        {
            public override string Name => "test";
            public override string? Version => null;
        }
        
        private class OtherTestPackage : BaseTestPackage
        {
            public override string Name => "unrelated";
            public override string? Version => "1.0.3";
        }
    }
}
