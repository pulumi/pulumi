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

        [Fact]
        public async Task TerminatesEarlyOnException()
        {
            var resources = await Deployment.TestAsync<TerminatesEarlyOnExceptionStack>(new EmptyMocks());
            var stack = (TerminatesEarlyOnExceptionStack)resources[0];
            var result = await stack.RunnerResult;
            Assert.Equal(0, result);
        }

        class TerminatesEarlyOnExceptionStack : Stack
        {
            public Task<int> RunnerResult { get; private set; }

            public TerminatesEarlyOnExceptionStack()
            {
                var runner = Pulumi.Deployment.Instance.Internal.Runner;
                var error = new Exception("Deliberate test error");

                Task task1 = Task.FromException(error);
                Task task2 = Task.Delay(100);

                runner.RegisterTask("task1", task1);
                runner.RegisterTask("task2", task2);

                var jointTask = runner.RunAsync<EmptyStack>();

                // The joinTask will await register tasks. We check here that it
                // finishes early because task1 failed, and does not wait for task2.
                this.RunnerResult = jointTask.ContinueWith(t => {
                    Assert.True(task1.IsFaulted, "task1 should be IsFaulted");
                    Assert.False(task2.IsCompleted, "task2 should not be IsCompleted yet");
                    Assert.Contains(error, runner.SwallowedExceptions);
                    return 0;
                });

                // Need to undo the recording of the error in the engine, otherwise
                // `Deployment.TestAsync` fails and does not let our test inspect
                // the RunnerResult promise.
                var deployment = (Deployment)Pulumi.Deployment.Instance.Internal;
                var engine = (MockEngine)deployment.Engine;
                engine.Errors.RemoveAll(err => err.Contains("Deliberate test error"));
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
