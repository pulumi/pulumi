// Copyright 2016-2020, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading.Tasks;
using Pulumi.Testing;
using Xunit;

namespace Pulumi.Tests.Mocks
{
    class MyMocks : IMocks
    {
        public Task<object> CallAsync(MockCallArgs args)
        {
            return Task.FromResult<object>(args);
        }

        public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args)
        {
            switch (args.Type)
            {
                case "aws:ec2/instance:Instance":
                    return Task.FromResult<(string?, object)>(("i-1234567890abcdef0", new Dictionary<string, object> {
                        { "publicIp", "203.0.113.12" },
                    }));
                case "pkg:index:MyCustom":
                    return Task.FromResult<(string?, object)>((args.Name + "_id",  args.Inputs));
                default:
                    throw new Exception($"Unknown resource {args.Type}");
            }
        }
    }

    public partial class MocksTests
    {
        [Fact]
        public async Task TestCustom()
        {
            var resources = await Testing.RunAsync<MyStack>();

            var instance = resources.OfType<Instance>().FirstOrDefault();
            Assert.NotNull(instance);

            var ip = await instance.PublicIp.GetValueAsync();
            Assert.Equal("203.0.113.12", ip);
        }

        [Fact]
        public async Task TestCustomWithResourceReference()
        {
            var resources = await Testing.RunAsync<MyStack>();

            var mycustom = resources.OfType<MyCustom>().FirstOrDefault();
            Assert.NotNull(mycustom);

            var instance = await mycustom.Instance.GetValueAsync();
            Assert.IsType<Instance>(instance);

            var ip = await instance.PublicIp.GetValueAsync();
            Assert.Equal("203.0.113.12", ip);
        }

        [Fact]
        public async Task TestStack()
        {
            var resources = await Testing.RunAsync<MyStack>();

            var stack = resources.OfType<MyStack>().FirstOrDefault();
            Assert.NotNull(stack);

            var ip = await stack.PublicIp.GetValueAsync();
            Assert.Equal("203.0.113.12", ip);
        }
    }

    public static class Testing
    {
        public static Task<ImmutableArray<Resource>> RunAsync<T>() where T : Stack, new()
        {
            return Deployment.TestAsync<T>(new MyMocks(), new TestOptions { IsPreview = false });
        }

        public static Task<T> GetValueAsync<T>(this Output<T> output)
        {
            var tcs = new TaskCompletionSource<T>();
            output.Apply(v =>
            {
                tcs.SetResult(v);
                return v;
            });
            return tcs.Task;
        }
    }
}
