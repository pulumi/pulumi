using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Pulumi.Serialization;
using Pulumi.Testing;

namespace Pulumi
{
    public sealed partial class Deployment
    {
        private class Tester : IDeploymentInternal, ILogger
        {
            private readonly ITestContext? _context;

            private readonly ILogger _logger;
            private readonly IRunner _runner;

            internal Stack? _stack;
            internal Stack Stack
            {
                get => _stack ?? throw new InvalidOperationException("Trying to acquire Deployment.Stack before 'Run' was called.");
                set => _stack = (value ?? throw new ArgumentNullException(nameof(value)));
            }

            public Tester(ITestContext? context)
            {
                _context = context;

                _runner = new Runner(this);
                _logger = this;
            }

            string IDeployment.ProjectName => "TestProject";
            string IDeployment.StackName => "TestStack";
            bool IDeployment.IsDryRun => _context?.IsDryRun ?? true;

            ILogger IDeploymentInternal.Logger => _logger;
            IRunner IDeploymentInternal.Runner => _runner;

            Stack IDeploymentInternal.Stack
            {
                get => Stack;
                set => Stack = value;
            }

            bool ILogger.LoggedErrors => _loggedErrors.Count > 0;

            private readonly List<string> _loggedErrors = new List<string>();
            private readonly List<Resource> _resources = new List<Resource>();

            public async Task<TestResult> TestAsync<TStack>() where TStack : Stack, new()
            {
                var result = await _runner.RunAsync<TStack>();
                return new TestResult(result > 0, _loggedErrors, _resources);
            }

            public string? GetConfig(string fullKey) => null;

            public void ReadOrRegisterResource(Resource resource, ResourceArgs args, ResourceOptions options)
            {
                _resources.Add(resource);

                if (!(resource is Stack))
                {
                    var completionSources = OutputCompletionSource.InitializeOutputs(resource);
                    foreach (var v in completionSources.Values)
                    {
                        v.SetValue(default);
                    }
                }

                _context?.ReadOrRegisterResource(resource, args, options);
            }

            public void RegisterResourceOutputs(Resource resource, Output<IDictionary<string, object?>> outputs)
            {
                _context?.RegisterResourceOutputs(resource, outputs);
            }

            public Task<T> InvokeAsync<T>(string token, InvokeArgs args, InvokeOptions? options = null)
                => _context?.InvokeAsync<T>(token, args, options)
                    ?? throw new InvalidOperationException($"Please supply an instance of '{nameof(ITestContext)}' with a stub '{nameof(InvokeAsync)}' implementation.");

            public Task InvokeAsync(string token, InvokeArgs args, InvokeOptions? options = null)
                => _context?.InvokeAsync(token, args, options) ?? Task.CompletedTask;

            public Task DebugAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
                => Task.CompletedTask;

            public Task InfoAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
                => Task.CompletedTask;

            public Task WarnAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
                => Task.CompletedTask;

            public Task ErrorAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            {
                _loggedErrors.Add(message);
                return Task.CompletedTask;
            }
        }
    }
}
