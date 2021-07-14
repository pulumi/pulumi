// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Threading.Tasks;
using Xunit;

using Pulumi.Testing;
using Pulumi.Tests.Mocks;

namespace Pulumi.Tests
{
    public class DeploymentRunerTests
    {
        [Fact]
        public async Task WorksUnderStress()
        {
            var resources = await Deployment.TestAsync<StressRunnerStack>(new EmptyMocks());
            var stack = (StressRunnerStack)resources[0];
            var result = await stack.RunnerResult;
            Assert.Equal(0, result);
        }

        class StressRunnerStack : Stack
        {
            public Task<int> RunnerResult { get; private set; }
            public StressRunnerStack()
            {
                var runner = Pulumi.Deployment.Instance.Internal.Runner;

                for (var i = 0; i < 100; i++)
                {
                    runner.RegisterTask($"task{i}", Task.Delay(100 + i));
                }

                this.RunnerResult = runner.RunAsync<EmptyStack>();
            }
        }

        class EmptyStack : Stack
        {
        }

        class EmptyMocks : IMocks
        {
            public Task<object> CallAsync(MockCallArgs args)
            {
                return Task.FromResult<object>(args);
            }

            public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args)
            {
                throw new Exception($"Unknown resource {args.Type}");
            }
        }
    }
}
