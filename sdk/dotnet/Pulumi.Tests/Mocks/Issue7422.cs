using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;

using Pulumi;
using Pulumi.Testing;

namespace Pulumi.Tests.Mocks.Issue7422
{
    /// <summary>
    /// It used to be possible to have a thread race causing the
    /// framework to observe null in the Urn property. The test
    /// attempts to increase the likelihood of the race by a few well
    /// placed `Task.Delay` and `Thread.Sleep`, but is in principle
    /// non-deterministic.
    ///
    /// See https://github.com/pulumi/pulumi/issues/7422
    /// </summary>
    class Issue7422Stack : Stack
    {
        public Issue7422Stack()
        {
            // Component Resource with delayed child (such as
            // Kubernetes ConfigGroup with children defined in yaml).
            // The child is provisioned in an apply, introducing
            // concurrency here.
            var comp1 = new Issue7422Component("comp1");

            // Any resource depending on the Component Resource. This
            // will race to find the child introduced in `apply`
            // before the child's constructor completes.
            new Issue7422Resource("res1", null, new CustomResourceOptions
            {
                DependsOn = comp1
            });
        }
    }

    public sealed class Issue7422ResourceArgs : ResourceArgs
    {
    }

    [ResourceType("issue7422::Resource", null)]
    public class Issue7422Resource : CustomResource
    {

        private Output<string> _someOutput = null!;

        [Output("someOutput")]
        public Output<string> SomeOutput
        {
            get
            {
                return _someOutput;
            }
            private set
            {
                if (GetResourceName() == "comp1-child")
                {
                    Thread.Sleep(5);
                }
                _someOutput = value;
            }
        }

        public Issue7422Resource(
            string name,
            Issue7422ResourceArgs? args = null,
            CustomResourceOptions? options = null)
            : base("issue7422::Resource", name, args, options)
        {
        }
    }

    public sealed class Issue7422Component : ComponentResource
    {
        public Issue7422Component(
            string name,
            ComponentResourceOptions? options = null)
            : base("issue7422::Component", name, options)
        {
            var parent = this;
            Output.Create(Later()).Apply(x =>
            {
                new Issue7422Resource($"{name}-child", null,
                                      new CustomResourceOptions()
                                      {
                                          Parent = parent,
                                      });
                return x;
            });
        }

        private async Task<int> Later()
        {
            await Task.Delay(2);
            return 1;
        }
    }

    class Issue7422Mocks : IMocks
    {
        public Task<object> CallAsync(MockCallArgs args)
        {
            return Task.FromResult<object>(args);
        }

        public async Task<(string? id, object state)> NewResourceAsync(
            MockResourceArgs args)
        {
            var emptyDict = new Dictionary<string, object>();
            (string?, object) empty = ($"i-{Guid.NewGuid()}", emptyDict);
            await Task.Delay(0);
            return args.Type switch
            {
                "issue7422::Component" => empty,
                "issue7422::Resource" => empty,
                _ => throw new Exception($"Unknown resource {args.Type}")
            };
        }
    }
}
