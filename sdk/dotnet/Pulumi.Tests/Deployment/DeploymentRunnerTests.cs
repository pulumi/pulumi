// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Linq;
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
        public async Task TerminatesEarlyOnException()
        {
            var deployResult = await Deployment.TryTestAsync<TerminatesEarlyOnExceptionStack>(new EmptyMocks());
            Assert.NotNull(deployResult.Exception);
            Assert.IsType<RunException>(deployResult.Exception!);
            Assert.Contains("Deliberate test error", deployResult.Exception!.Message);
            var stack = (TerminatesEarlyOnExceptionStack)deployResult.Resources[0];
            Assert.False(stack.SlowOutput.GetValueAsync(whenUnknown: default!).IsCompleted);
        }

        class TerminatesEarlyOnExceptionStack : Stack
        {
            [Output]
            public Output<int> SlowOutput { get; private set; }

            public TerminatesEarlyOnExceptionStack()
            {
                Output.Create(Task.FromException<int>(new Exception("Deliberate test error")));
                SlowOutput = Output.Create(Task.Delay(60000).ContinueWith(_ => 1));
            }
        }

        [Fact]
        public async Task LogsTaskDescriptions()
        {
            var resources = await Deployment.TestAsync<LogsTaskDescriptionsStack>(new EmptyMocks());
            var stack = (LogsTaskDescriptionsStack)resources[0];
            var logger = await stack.Logger;

            // Logs are fire-and-forget, and we poll here until it propagates.
            var pollAttempt = 0;
            while (logger.Messages.Count() < 4)
            {
                await Task.Delay(10);
                pollAttempt++;
                if (pollAttempt == 20)
                {
                    break;
                }
            }

            var messages = logger.Messages;

            for (var i = 0; i < 2; i++)
            {
                Assert.Contains($"Debug 0 Registering task: task{i}", messages);
                Assert.Contains($"Debug 0 Completed task: task{i}", messages);
            }
        }

        class LogsTaskDescriptionsStack : Stack
        {
            public Task<InMemoryLogger> Logger { get; private set; }

            public LogsTaskDescriptionsStack()
            {
                var deployment = Pulumi.Deployment.Instance.Internal;
                var logger = new InMemoryLogger();
                var runner = new Deployment.Runner(deployment, logger);

                for (var i = 0; i < 2; i++)
                {
                    runner.RegisterTask($"task{i}", Task.Delay(100 + i));
                }

                this.Logger = runner.WhileRunningAsync().ContinueWith(_ => logger);
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

            public IEnumerable<String> Messages
            {
                get
                {
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
