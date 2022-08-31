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

        public static string InstancePublicIpAddress => "203.0.113.12";

        public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args) =>
            args.Type switch
            {
                "aws:ec2/instance:Instance" => Task.FromResult<(string?, object)>(("i-1234567890abcdef0", new Dictionary<string, object> { { "publicIp", InstancePublicIpAddress  }, })),
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

    class MyInvalidMocks : IMocks
    {
        public Task<object> CallAsync(MockCallArgs args)
        {
            return Task.FromResult<object>(args);
        }

        public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args) =>
            args.Type switch
            {
                "aws:ec2/instance:Instance" => Task.FromResult<(string?, object)>(("i-1234567890abcdef0", new Dictionary<string, object> { { "publicIp", unchecked((int)0xcb00710c) }, })),
                "pkg:index:MyCustom" => Task.FromResult<(string?, object)>((args.Name + "_id", args.Inputs)),
                _ => throw new Exception($"Unknown resource {args.Type}")
            };
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
            var (resources, exception) = await Testing.RunAsync(new Issue8163Mocks(), async () =>
            {

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

        [Fact]
        public async Task TestInvokeToleratesUnknownsInPreview()
        {
            var resources = await Deployment.TestAsync<Issue8322.ReproStack>(
                new Issue8322.ReproMocks(),
                new TestOptions() { IsPreview = true }
            );
            var stack = resources.OfType<Issue8322.ReproStack>().Single();
            var result = await stack.Result.GetValueAsync(whenUnknown: "unknown!");
            Assert.Equal("unknown!", result);
        }

        [Fact]
        public async Task TestStackWithInvalidSchema()
        {
            var resources = await Deployment.TestAsync<MyStack>(new MyInvalidMocks(), new TestOptions { IsPreview = false });

            var stack = resources.OfType<MyStack>().FirstOrDefault();
            Assert.NotNull(stack);

            var ip = await stack!.PublicIp.GetValueAsync(whenUnknown: default!);
            Assert.Null(ip);

            // TODO: It would be good to assert that a warning was logged to the engine but getting hold of warnings requires re-plumbing what TestAsync returns.
        }

        private class NullOutputStack : Stack
        {
            [Output("foo")]
            public Output<string>? Foo { get; } = null;
        }

        [Fact]
        public async Task StackWithNullOutputsThrows()
        {
            try
            {
                await Testing.RunAsync<NullOutputStack>();
            }
            catch (Exception ex)
            {
                Assert.Contains(
                    "System.InvalidOperationException: " +
                    "[Output] Pulumi.Tests.Mocks.MocksTests+NullOutputStack.Foo " +
                    "did not have a 'set' method",
                    ex.ToString());
                return;
            }

            throw new Exception("Expected to fail");
        }

        [Fact]
        public async Task TestUrnOutputPropertyIsNeverNull()
        {
            await Deployment.TestAsync<Issue7422.Issue7422Stack>(
                new Issue7422.Issue7422Mocks());
        }

        [Fact]
        public async Task TestAliases()
        {
            var resources = await Deployment.TestAsync<Aliases.AliasesStack>(
                new Aliases.AliasesMocks());

            // TODO[pulumi/pulumi#8637]
            //
            // var parent1Urn = await resources[1].Urn.GetValueAsync("");
            // Assert.Equal("urn:pulumi:stack::project::test:resource:type::myres1", parent1Urn);

            // TODO[pulumi/pulumi#8637]: A subset of the "expected" below include the implicit root stack type
            // `pulumi:pulumi:Stack` as an explicit parent type in the URN. This should not happen, and indicates
            // a bug in the the Pulumi .NET SDK unrelated to Aliases.  It appears this only happens when using the
            // .NET mock testing framework, not when running normal programs.
            var expected = new Dictionary<string, List<string>>{
                { "myres1-child", new List<string>{}},
                { "myres2-child", new List<string>{
                    "urn:pulumi:stack::project::pulumi:pulumi:Stack$test:resource:type$test:resource:child2::myres2-child"
                }},
                { "myres3-child", new List<string>{
                    "urn:pulumi:stack::project::pulumi:pulumi:Stack$test:resource:type$test:resource:child::child2"
                }},
                { "myres4-child", new List<string>{
                    "urn:pulumi:stack::project::pulumi:pulumi:Stack$test:resource:type$test:resource:child::myres4-child2",
                    "urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres4-child",
                    "urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres4-child2",
                }},
                { "myres5-child", new List<string>{
                    "urn:pulumi:stack::project::pulumi:pulumi:Stack$test:resource:type$test:resource:child::myres5-child2",
                    "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres52-child",
                    "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres52-child2",
                }},
                { "myres6-child", new List<string>{
                    "urn:pulumi:stack::project::pulumi:pulumi:Stack$test:resource:type$test:resource:child::myres6-child2",
                    "urn:pulumi:stack::project::pulumi:pulumi:Stack$test:resource:type$test:resource:child2::myres6-child",
                    "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres62-child",
                    "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres62-child2",
                    "urn:pulumi:stack::project::test:resource:type$test:resource:child2::myres62-child",
                    "urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres6-child",
                    "urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres6-child2",
                    "urn:pulumi:stack::project::test:resource:type3$test:resource:child2::myres6-child",
                    "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres63-child",
                    "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres63-child2",
                    "urn:pulumi:stack::project::test:resource:type$test:resource:child2::myres63-child",
                }},

            };
            foreach (var resource in resources)
            {
                if (resource.GetResourceType() == "test:resource:child")
                {
                    var aliases = await Output.All(resource._aliases).GetValueAsync(ImmutableArray.Create<string>());
                    Assert.Equal<string>(expected[resource.GetResourceName()], aliases);
                }
            }
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
