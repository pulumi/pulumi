// Copyright 2016-2021, Pulumi Corporation

using System.Threading.Tasks;
using Xunit;

namespace Pulumi.Tests
{
    public class DeploymentRunerTests
    {
        [Fact]
        public async Task WorksUnderStress()
        {
            var resources = await Deployment.TestAsync<StressRunnerStack>(new AntonMocks());
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
    }
}