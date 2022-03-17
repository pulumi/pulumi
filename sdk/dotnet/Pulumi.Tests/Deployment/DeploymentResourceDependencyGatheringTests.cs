// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;

using Microsoft.Extensions.Logging;
using Xunit;

using Pulumi.Testing;
using Pulumi.Tests.Mocks;

namespace Pulumi.Tests
{
    public class DeploymentResourceDependencyGatheringTests
    {
        [Fact]
        public async Task DeploysResourcesWithUnknownDependsOn()
        {
            var deployResult = await Deployment.TryTestAsync<DeploysResourcesWithUnknownDependsOnStack>(
                new EmptyMocks(isPreview: true),
                new TestOptions()
                {
                    IsPreview = true,
                });
            Assert.Null(deployResult.Exception);
        }

        class DeploysResourcesWithUnknownDependsOnStack : Stack
        {
            public DeploysResourcesWithUnknownDependsOnStack()
            {
                new MyCustomResource("r1", null, new CustomResourceOptions()
                {
                    DependsOn = Output<Resource[]>.CreateUnknown(new Resource[]{}),
                });
            }
        }

        public sealed class MyArgs : ResourceArgs
        {
        }

        [ResourceType("test:DeploymentResourceDependencyGatheringTests:resource", null)]
        private class MyCustomResource : CustomResource
        {
            public MyCustomResource(string name, MyArgs? args, CustomResourceOptions? options = null)
                : base("test:DeploymentResourceDependencyGatheringTests:resource", name, args ?? new MyArgs(), options)
            {
            }
        }

        class EmptyMocks : IMocks
        {
            public bool IsPreview { get; private set; }

            public EmptyMocks(bool isPreview)
            {
                this.IsPreview = isPreview;
            }

            public Task<object> CallAsync(MockCallArgs args)
            {
                return Task.FromResult<object>(args);
            }

            public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args)
            {
                if (args.Type == "test:DeploymentResourceDependencyGatheringTests:resource")
                {
                    return Task.FromResult<(string?, object)>((this.IsPreview ? null : "id",
                                                               new Dictionary<string, object>()));
                }
                throw new Exception($"Unknown resource {args.Type}");
            }
        }
    }
}
