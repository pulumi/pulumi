// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Threading.Tasks;

using Microsoft.Extensions.Logging;
using Xunit;

using Pulumi.Testing;
using Pulumi.Tests.Mocks;

namespace Pulumi.Tests
{
    public class DeploymentRunnerTests
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
            var deployResult = await Deployment.TryTestAsync<TerminatesEarlyOnExceptionStack>(new EmptyMocks());
            Assert.NotNull(deployResult.Exception);
            if (deployResult.Exception! is AggregateException)
            {
                // The "Deliberate test error" should really be thrown
                // only once, but unfortunately observing the multiple
                // exceptions case in CI, so adding this clause to
                // ensure all of them are test errors, and we just
                // have unintentional duplication.
                var e = (AggregateException)deployResult.Exception!;
                foreach (var err in e.InnerExceptions)
                {
                    Assert.IsType<RunException>(err);
                    Assert.Contains("Deliberate test error", err.Message);
                }
            }
            else
            {
                Assert.IsType<RunException>(deployResult.Exception!);
                Assert.Contains("Deliberate test error", deployResult.Exception!.Message);
            }

            var stack = (TerminatesEarlyOnExceptionStack)deployResult.Resources[0];
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

                runner.RegisterTask("task2", task2);
                runner.RegisterTask("task1", task1);

                var jointTask = runner.RunAsync<EmptyStack>();

                // The joinTask will await register tasks. We check here that it
                // finishes early because task1 failed, and does not wait for task2.
                this.RunnerResult = jointTask.ContinueWith(t => {
                    Assert.True(task1.IsFaulted, "task1 should be IsFaulted");
                    Assert.False(task2.IsCompleted, "task2 should not be IsCompleted yet");
                    Assert.Contains(error, runner.SwallowedExceptions);
                    return 0;
                });
            }
        }

        [Fact]
        public async Task LogsTaskDescriptions()
        {
            var resources = await Deployment.TestAsync<LogsTaskDescriptionsStack>(new EmptyMocks());
            var stack = (LogsTaskDescriptionsStack)resources[0];
            var messages = await stack.Logs;
            for (var i = 0; i < 2; i++)
            {
                Assert.Contains($"Debug 0 Registering task: task{i}", messages);
                Assert.Contains($"Debug 0 Completed task: task{i}", messages);
            }
        }

        class LogsTaskDescriptionsStack : Stack
        {
            public Task<IEnumerable<string>> Logs { get; private set; }

            public LogsTaskDescriptionsStack()
            {
                var deployment = Pulumi.Deployment.Instance.Internal;
                var logger = new InMemoryLogger();
                var runner = new Deployment.Runner(deployment, logger);

                for (var i = 0; i < 2; i++)
                {
                    runner.RegisterTask($"task{i}", Task.Delay(100 + i));
                }

                this.Logs = ((IRunner)runner).RunAsync<EmptyStack>().ContinueWith(_ => logger.Messages);
            }
        }

        class InMemoryLogger : ILogger
        {
            private readonly object _lockObject = new object();
            private readonly List<string> _messages = new List<string>();

            public void Log<TState>(
                LogLevel level,
                EventId eventId,
                TState state,
                Exception exc,
                Func<TState, Exception, string> formatter)
            {
                var msg = formatter(state, exc);
                Write($"{level} {eventId} {msg}");
            }

            public IEnumerable<String> Messages {
                get {
                    lock (_lockObject)
                    {
                        return _messages.ToArray();
                    }
                }
            }

            public IDisposable BeginScope<TState>(TState state)
            {
                Write($"BeginScope state={state}");
                return new Scope()
                {
                    Close = () => {
                        Write($"EndScope state={state}");
                    }
                };
            }

            public bool IsEnabled(LogLevel level)
            {
                return true;
            }

            private void Write(string message)
            {
                lock (_lockObject)
                {
                    _messages.Add(message);
                }
            }

            class Scope : IDisposable
            {
                public Action Close { get; set; } = () => {};

                public void Dispose()
                {
                    Close();
                }
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
