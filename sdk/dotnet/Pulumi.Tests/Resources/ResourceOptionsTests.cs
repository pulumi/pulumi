// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading.Tasks;
using Xunit;

using Pulumirpc;
using Pulumi.Testing;

namespace Pulumi.Tests.Resources
{
    public class ResourceOptionsTests
    {
        [Fact]
        public async Task DependsOnAcceptsInputs()
        {
            var resources = await Testing.RunAsync<DependsOnAcceptsInputsStack>();

            var stack = resources.OfType<DependsOnAcceptsInputsStack>().FirstOrDefault();
            Assert.NotNull(stack);
        }

        class DependsOnAcceptsInputsStack : Stack
        {
            public DependsOnAcceptsInputsStack()
            {
                var arg = new TestResourceArgs() { N = 1 };
                new TestResource("r1", arg);
            }
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

    static class Testing
    {
        public static Task<ImmutableArray<Resource>> RunAsync<T>() where T : Stack, new()
        {
            var opts = new TestOptions { IsPreview = false };
            var internalTestOpts = Deployment.InternalTestOptions.Create(opts);
            internalTestOpts.ConfigureMonitor = m => new DependencyTrackingMonitor(m);
            return Deployment.TestAsync<T>(new MyMocks(), internalTestOpts);
        }
    }

    class DependencyTrackingMonitor : IMockMonitor, IMonitor
    {
        private IMockMonitor Inner { get; }

        public DependencyTrackingMonitor(IMockMonitor inner)
        {
            Inner = inner;
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
            Console.WriteLine($"resp.URN = {resp.Urn}; req.DEps = ${request.Dependencies}");
            return resp;
        }

        async Task IMonitor.RegisterResourceOutputsAsync(RegisterResourceOutputsRequest request)
        {
            await Inner.RegisterResourceOutputsAsync(request);
        }
    }
}


// func TestDependsOnInputs(t *testing.T) {
// 	t.Run("known", func(t *testing.T) {
// 		err := RunErr(func(ctx *Context) error {
// 			depTracker := trackDependencies(ctx)

// 			dep1 := newTestRes(t, ctx, "dep1")
// 			dep2 := newTestRes(t, ctx, "dep2")

// 			output := outputDependingOnResource(dep1, true).
// 				ApplyT(func(int) Resource { return dep2 }).(ResourceOutput)

// 			res := newTestRes(t, ctx, "res", DependsOnInputs([]ResourceInput{output}))
// 			assertHasDeps(t, ctx, depTracker, res, dep1, dep2)
// 			return nil
// 		}, WithMocks("project", "stack", &testMonitor{}))
// 		assert.NoError(t, err)
// 	})
// 	t.Run("unknown", func(t *testing.T) {
// 		err := RunErr(func(ctx *Context) error {
// 			depTracker := trackDependencies(ctx)

// 			dep1 := newTestRes(t, ctx, "dep1")
// 			dep2 := newTestRes(t, ctx, "dep2")
// 			dep3 := newTestRes(t, ctx, "dep3")

// 			out := outputDependingOnResource(dep1, true).
// 				ApplyT(func(int) Resource { return dep2 }).(ResourceOutput)

// 			out2 := outputDependingOnResource(dep3, false).
// 				ApplyT(func(int) Resource { return dep2 }).(ResourceOutput)

// 			res := newTestRes(t, ctx, "res", DependsOnInputs([]ResourceInput{out, out2}))
// 			assertHasDeps(t, ctx, depTracker, res, dep1, dep2, dep3)
// 			return nil
// 		}, WithMocks("project", "stack", &testMonitor{}))
// 		assert.NoError(t, err)
// 	})
// }

// func TestDependsOnOutput(t *testing.T) {
// 	err := RunErr(func(ctx *Context) error {
// 		depTracker := trackDependencies(ctx)

// 		anyOut := func(value interface{}) AnyOutput {
// 			out, resolve, _ := ctx.NewOutput()
// 			resolve(value)
// 			return out.(AnyOutput)
// 		}

// 		checkDeps := func(name string, dependsOn AnyOutput, expectedDeps ...Resource) {
// 			res := newTestRes(t, ctx, name, DependsOnOutput(dependsOn))
// 			assertHasDeps(t, ctx, depTracker, res, expectedDeps...)
// 		}

// 		dep1 := newTestRes(t, ctx, "dep1")
// 		dep2 := newTestRes(t, ctx, "dep2")
// 		dep3 := newTestRes(t, ctx, "dep3")

// 		out := outputDependingOnResource(dep1, true).
// 			ApplyT(func(int) Resource { return dep2 }).(ResourceOutput)

// 		checkDeps("r1", anyOut([]Resource{dep1, dep2}), dep1, dep2)
// 		checkDeps("r2", anyOut([]ResourceInput{out}), dep1, dep2)
// 		checkDeps("r3", anyOut([]interface{}{out, dep3}), dep1, dep2, dep3)

// 		dep4 := newTestRes(t, ctx, "dep4")
// 		out4 := outputDependingOnResource(dep4, true).
// 			ApplyT(func(int) AnyOutput { return anyOut([]Resource{dep1, dep2}) }).(AnyOutput)
// 		checkDeps("r4", out4, dep1, dep2, dep4)

// 		return nil
// 	}, WithMocks("project", "stack", &testMonitor{}))
// 	assert.NoError(t, err)
// }
