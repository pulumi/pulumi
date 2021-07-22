// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading.Tasks;
using Xunit;

using Pulumirpc;
using Pulumi.Serialization;
using Pulumi.Testing;


namespace Pulumi.Tests.Resources
{
    using static Testing;

    public class ResourceOptionsTests
    {
        [Fact]
        public async Task DependsOnRespectsDirectOutput()
        {
            var results = await Testing.RunAsync<DependsOnRespectsDirectOutputStack>();
            AssertDependsOn(results.Dependencies, "res", "dep1");
        }

        class DependsOnRespectsDirectOutputStack : Stack
        {
            public DependsOnRespectsDirectOutputStack()
            {
                var dep1 = NewResource("dep1", 1);
                NewResource("res", 3, DependsOn(Output.Create(dep1)));
            }
        }

        [Fact]
        public async Task DependsOnRespectsIndirectOutput()
        {
            var results = await Testing.RunAsync<DependsOnRespectsIndirectOutputStack>();
            AssertDependsOn(results.Dependencies, "res", "dep2");
        }

        class DependsOnRespectsIndirectOutputStack : Stack
        {
            public DependsOnRespectsIndirectOutputStack()
            {
                var dep1 = NewResource("dep1", 1);
                var dep2 = NewResource("dep2", 2);
                var dep = OutputDependingOnResource(dep2, isKnown: true).Apply(_ => dep1);
                NewResource("res", 3, DependsOn(dep));
            }
        }

        [Fact]
        public async Task DependsOnDoesNotLoseInfoWithUnknowns()
        {
            var results = await Testing.RunAsync<DependsOnDoesNotLoseInfoWithUnknownsStack>(isPreview: true);
            AssertDependsOn(results.Dependencies, "res", "dep1");
        }

        class DependsOnDoesNotLoseInfoWithUnknownsStack : Stack
        {
            public DependsOnDoesNotLoseInfoWithUnknownsStack()
            {
                var dep1 = NewResource("dep1", 1);
                var dep2 = NewResource("dep2", 2);
                var dep3 = NewResource("dep3", 3);
                var known = Output.Create(dep1);
                var unknown = OutputDependingOnResource(dep3, isKnown: false).Apply(_ => dep2);
                NewResource("res", 4, DependsOn(known, unknown));
            }
        }

        [Fact]
        public async Task DependsOnRespectsIndirectTopLevelOutput()
        {
            var results = await Testing.RunAsync<DependsOnRespectsIndirectTopLevelOutputStack>();
            AssertDependsOn(results.Dependencies, "res", "dep2");
        }

        class DependsOnRespectsIndirectTopLevelOutputStack : Stack
        {
            public DependsOnRespectsIndirectTopLevelOutputStack()
            {
                var dep1 = NewResource("dep1", 1);
                var dep2 = NewResource("dep2", 2);
                Output<IEnumerable<Resource>> dep = OutputDependingOnResource(dep2, isKnown: true)
                    .Apply(_ => Enumerable.Repeat(dep1, 1));
                NewResource("res", 3, DependsOn(dep));
            }
        }
    }

    static class Testing
    {
        public static CustomResourceOptions DependsOn(params Output<Resource>[] resources)
        {
            var opts = new CustomResourceOptions();
            foreach (var dep in resources)
            {
                opts.DependsOn.Add(dep);
            }
            return opts;
        }

        public static CustomResourceOptions DependsOn(Output<IEnumerable<Resource>> resources)
        {
            var opts = new CustomResourceOptions();
            opts.DependsOn = resources;
            return opts;
        }

        sealed class MyMocks : IMocks
        {
            public Task<object> CallAsync(MockCallArgs args)
            {
                return Task.FromResult<object>(args);
            }

            public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args)
            {
                return Task.FromResult<(string?, object)>((args.Name + "_id", args.Inputs));
            }
        }

        public sealed class TestResourceArgs : ResourceArgs
        {
            [Input("n", required: true)]
            public Input<int> N { get; set; } = null!;

            public TestResourceArgs()
            {
            }
        }

        sealed class TestResource : CustomResource
        {
            [Output("nextInteger")]
            public Output<int> NextInteger { get; private set; } = null!;

            public TestResource(string name, TestResourceArgs args, CustomResourceOptions? options = null)
		: base("test:index:TestResource", name, args, options)
            {
                NextInteger = args.N.Apply(n => n + 1);
            }
        }

        /// <summary>
        /// Like RunAsync, but also helps the test track dependencies
        /// as a mapping from a resource URN to a list of URNs
        /// identifying resources it depends on.
        /// </summary>
        public static async Task<(
            ImmutableArray<Resource> Resources,
            ImmutableDictionary<string,ImmutableList<string>> Dependencies
        )> RunAsync<T>(bool isPreview = false) where T : Stack, new()
        {
            var deps = new ConcurrentDictionary<string,ImmutableList<string>>();
            var opts = new TestOptions { IsPreview = isPreview };
            var internalTestOpts = Deployment.InternalTestOptions.Create(opts);
            internalTestOpts.ConfigureMonitor = m => new DependencyTrackingMonitor(m, deps);
            var resources = await Deployment.TestAsync<T>(new MyMocks(), internalTestOpts);

            return (Resources: resources, Dependencies: deps.ToImmutableDictionary());
        }

        public static void AssertDependsOn(ImmutableDictionary<string,ImmutableList<string>> dependencies,
                                           string resourceName,
                                           string dependentResourceName)
        {
            string key = dependencies.Keys.Where(urn => urn.EndsWith(resourceName)).FirstOrDefault();
            Assert.NotNull(key);
            Assert.Contains(dependencies[key], urn => urn.EndsWith(dependentResourceName));
        }

        public static Resource NewResource(string name, int n, CustomResourceOptions? opts = null)
        {
            return new TestResource(name, new TestResourceArgs() { N = n }, opts);
        }

        public static Output<int> OutputDependingOnResource(Resource resource, bool isKnown)
        {
            var od = new OutputData<int>(ImmutableHashSet.Create(resource), 0, isKnown, false);
            return new Output<int>(Task.FromResult(od));
        }

        class DependencyTrackingMonitor : IMockMonitor, IMonitor
        {
            private IMockMonitor Inner { get; }
            private ConcurrentDictionary<string, ImmutableList<string>> Dependencies { get; }

            public DependencyTrackingMonitor(IMockMonitor inner, ConcurrentDictionary<string, ImmutableList<string>> deps)
            {
                Inner = inner;
                Dependencies = deps;
            }

            ImmutableList<Resource> IMockMonitor.Resources
            {
                get
                {
                    return Inner.Resources;
                }
            }

            async Task<SupportsFeatureResponse> IMonitor.SupportsFeatureAsync(SupportsFeatureRequest request)
            {
                return await Inner.SupportsFeatureAsync(request);
            }

            async Task<InvokeResponse> IMonitor.InvokeAsync(InvokeRequest request)
            {
                return await Inner.InvokeAsync(request);
            }

            async Task<ReadResourceResponse> IMonitor.ReadResourceAsync(Resource resource, ReadResourceRequest request)
            {
                return await Inner.ReadResourceAsync(resource, request);
            }

            async Task<RegisterResourceResponse> IMonitor.RegisterResourceAsync(Resource resource, RegisterResourceRequest request)
            {
                var resp = await Inner.RegisterResourceAsync(resource, request);
                Console.WriteLine($"Registered {resp.Urn}");
                var deps = request.Dependencies.ToImmutableList();
                Dependencies.AddOrUpdate(resp.Urn,
                                         _ => request.Dependencies.Distinct().ToImmutableList(),
                                         (_, oldDeps) => oldDeps.Concat(deps).Distinct().ToImmutableList());
                return resp;
            }

            async Task IMonitor.RegisterResourceOutputsAsync(RegisterResourceOutputsRequest request)
            {
                await Inner.RegisterResourceOutputsAsync(request);
            }
        }
    }
}
