// Copyright 2016-2021, Pulumi Corporation

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

        public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args) =>
            args.Type switch
            {
                "aws:ec2/instance:Instance" => Task.FromResult<(string?, object)>(("i-1234567890abcdef0", new Dictionary<string, object> { { "publicIp", "203.0.113.12" }, })),
                "pkg:index:MyCustom" => Task.FromResult<(string?, object)>((args.Name + "_id", args.Inputs)),
                _ => throw new Exception($"Unknown resource {args.Type}")
            };
    }

    class Issue8163Mocks : IMocks
    {
        public Task<object> CallAsync(MockCallArgs args)
        {
            throw new Grpc.Core.RpcException(new Grpc.Core.Status(Grpc.Core.StatusCode.Unknown, "error code 404"));
        }

        public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args) => throw new Exception("Not used");
    }

    public class MocksTests
    {
        [Fact]
        public async Task TestCustom()
        {
            var resources = await Testing.RunAsync<MyStack>();

            var instance = resources.OfType<Instance>().FirstOrDefault();
            Assert.NotNull(instance);

            var ip = await instance!.PublicIp.GetValueAsync(whenUnknown: default!);
            Assert.Equal("203.0.113.12", ip);
        }

        [Fact]
        public async Task TestCustomWithResourceReference()
        {
            var resources = await Testing.RunAsync<MyStack>();

            var myCustom = resources.OfType<MyCustom>().FirstOrDefault();
            Assert.NotNull(myCustom);

            var instance = await myCustom!.Instance.GetValueAsync(whenUnknown: default!);
            Assert.IsType<Instance>(instance);

            var ip = await instance.PublicIp.GetValueAsync(whenUnknown: default!);
            Assert.Equal("203.0.113.12", ip);
        }

        [Fact]
        public async Task TestStack()
        {
            var resources = await Testing.RunAsync<MyStack>();

            var stack = resources.OfType<MyStack>().FirstOrDefault();
            Assert.NotNull(stack);

            var ip = await stack!.PublicIp.GetValueAsync(whenUnknown: default!);
            Assert.Equal("203.0.113.12", ip);
        }

        /// Test for https://github.com/pulumi/pulumi/issues/8163
        [Fact]
        public async Task TestInvokeThrowing()
        {
            var (resources, exception) = await Testing.RunAsync(new Issue8163Mocks(), async () => {

                var role = await GetRole.InvokeAsync(new GetRoleArgs()
                {
                    Name = "doesNotExistTypoEcsTaskExecutionRole"
                });

                var myInstance = new Instance("instance", new InstanceArgs());

                return new Dictionary<string, object?>()
                {
                    { "result", "x"},
                    { "instance", myInstance.PublicIp }
                };
            });

            var stack = resources.OfType<Stack>().FirstOrDefault();
            Assert.NotNull(stack);

            var instance = resources.OfType<Instance>().FirstOrDefault();
            Assert.Null(instance);

            Assert.NotNull(exception);
            Assert.StartsWith("Running program '", exception!.Message);
            Assert.Contains("' failed with an unhandled exception:", exception!.Message);
            Assert.Contains("Grpc.Core.RpcException: Status(StatusCode=\"Unknown\", Detail=\"error code 404\")", exception!.Message);
        }
    }

    public static class Testing
    {
        public static Task<ImmutableArray<Resource>> RunAsync<T>() where T : Stack, new()
        {
            return Deployment.TestAsync<T>(new MyMocks(), new TestOptions { IsPreview = false });
        }

        public static Task<(ImmutableArray<Resource> Resources, Exception? Exception)> RunAsync(IMocks mocks, Func<Task<IDictionary<string, object?>>> func)
        {
            return Deployment.TryTestAsync(mocks, runner => runner.RunAsync(func, null), new TestOptions { IsPreview = false });
        }
    }
}
