using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;

using Pulumi;
using Pulumi.Testing;

namespace Pulumi.Tests.Mocks.Aliases
{
    class AliasesStack : Stack
    {
        public AliasesStack()
        {
            var parent1 = new Pulumi.CustomResource("test:resource:type", "myres1", null, new CustomResourceOptions { });
            var child1 = new Pulumi.CustomResource("test:resource:child", "myres1-child", null, new CustomResourceOptions
            {
                Parent = parent1,
            });

            var parent2 = new Pulumi.CustomResource("test:resource:type", "myres2", null, new CustomResourceOptions { });
            var child2 = new Pulumi.CustomResource("test:resource:child", "myres2-child", null, new CustomResourceOptions
            {
                Parent = parent2,
                Aliases = { new Alias { Type = "test:resource:child2" } }
            });

            var parent3 = new Pulumi.CustomResource("test:resource:type", "myres3", null, new CustomResourceOptions { });
            var child3 = new Pulumi.CustomResource("test:resource:child", "myres3-child", null, new CustomResourceOptions
            {
                Parent = parent3,
                Aliases = { new Alias { Name = "child2" } }
            });

            var parent4 = new Pulumi.CustomResource("test:resource:type", "myres4", null, new CustomResourceOptions
            {
                Aliases = { new Alias { Type = "test:resource:type3" } }
            });
            var child4 = new Pulumi.CustomResource("test:resource:child", "myres4-child", null, new CustomResourceOptions
            {
                Parent = parent4,
                Aliases = { new Alias { Name = "myres4-child2" } }
            });

            var parent5 = new Pulumi.CustomResource("test:resource:type", "myres5", null, new CustomResourceOptions
            {
                Aliases = { new Alias { Name = "myres52" } }
            });
            var child5 = new Pulumi.CustomResource("test:resource:child", "myres5-child", null, new CustomResourceOptions
            {
                Parent = parent5,
                Aliases = { new Alias { Name = "myres5-child2" } }
            });

            var parent6 = new Pulumi.CustomResource("test:resource:type", "myres6", null, new CustomResourceOptions
            {
                Aliases = {  new Alias { Name = "myres62" }, new Alias { Type = "test:resource:type3"}, new Alias { Name = "myres63" }, }
            });
            var child6 = new Pulumi.CustomResource("test:resource:child", "myres6-child", null, new CustomResourceOptions
            {
                Parent = parent6,
                Aliases = { new Alias { Name = "myres6-child2" }, new Alias { Type = "test:resource:child2"} }
            });
        }
    }

    class AliasesMocks : IMocks
    {
        public Task<object> CallAsync(MockCallArgs args)
        {
            return Task.FromResult<object>(args);
        }

        public async Task<(string? id, object state)> NewResourceAsync(
            MockResourceArgs args)
        {
            await Task.Delay(0);
            return ("myID", new Dictionary<string, object>());
        }
    }
}
