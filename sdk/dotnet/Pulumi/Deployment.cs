// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Grpc.Core;
using Pulumirpc;

namespace Pulumi
{
    public partial class Deployment
    {
        private static Deployment? _instance;
        public static Deployment Instance
        {
            get => _instance ?? throw new InvalidOperationException("Trying to acquire Deployment.Instance before 'Run' was called.");
            set => _instance = (value ?? throw new ArgumentNullException(nameof(value)));
        }

        private readonly Queue<(string description, Task task)> _tasks =
            new Queue<(string description, Task task)>();

        public Options Options { get; }
        internal Engine.EngineClient Engine { get; }
        internal ResourceMonitor.ResourceMonitorClient Monitor { get; }

        internal Stack? _stack;
        public Stack Stack
        {
            get => _stack ?? throw new InvalidOperationException("Trying to acquire Deployment.Stack before 'Run' was called.");
            set => _stack = (value ?? throw new ArgumentNullException(nameof(value)));
        }

        private Deployment()
        {
            var monitor = Environment.GetEnvironmentVariable("PULUMI_MONITOR");
            var engine = Environment.GetEnvironmentVariable("PULUMI_ENGINE");
            var project = Environment.GetEnvironmentVariable("PULUMI_PROJECT");
            var stack = Environment.GetEnvironmentVariable("PULUMI_STACK");
            var pwd = Environment.GetEnvironmentVariable("PULUMI_PWD");
            var dryRun = Environment.GetEnvironmentVariable("PULUMI_DRY_RUN");
            var queryMode = Environment.GetEnvironmentVariable("PULUMI_QUERY_MODE");
            var parallel = Environment.GetEnvironmentVariable("PULUMI_PARALLEL");
            var tracing = Environment.GetEnvironmentVariable("PULUMI_TRACING");
            var config = Environment.GetEnvironmentVariable("PULUMI_CONFIG");

            if (string.IsNullOrEmpty(monitor))
                throw new InvalidOperationException("Environment did not contain: PULUMI_MONITOR");

            if (string.IsNullOrEmpty(engine))
                throw new InvalidOperationException("Environment did not contain: PULUMI_ENGINE");

            if (!bool.TryParse(dryRun, out var dryRunValue))
                throw new InvalidOperationException("Environment did not contain a valid bool value for: PULUMI_DRY_RUN");

            if (!bool.TryParse(queryMode, out var queryModeValue))
                throw new InvalidOperationException("Environment did not contain a valid bool value for: PULUMI_QUERY_MODE");

            if (!int.TryParse(parallel, out var parallelValue))
                throw new InvalidOperationException("Environment did not contain a valid int value for: PULUMI_PARALLEL");

            this.Options = new Options(
                dryRun: dryRunValue, queryMode: queryModeValue, parallel: parallelValue,
                project: project, stack: stack, pwd: pwd,
                monitor: monitor, engine: engine, tracing: tracing);

            this.Engine = new Engine.EngineClient(new Channel(engine, ChannelCredentials.Insecure));
            this.Monitor = new ResourceMonitor.ResourceMonitorClient(new Channel(monitor, ChannelCredentials.Insecure));
        }

        public static Task Run(Action action)
            => Run(() =>
            {
                action();
                return ImmutableDictionary<string, object>.Empty;
            });

        public static Task Run(Func<IDictionary<string, object>> func)
            => Run(() => Task.FromResult(func()));

        public static Task Run(Func<Task<IDictionary<string, object>>> func)
        {
            if (Instance != null)
            {
                throw new NotSupportedException("Deployment.Run can only be called a single time.");
            }

            Instance = new Deployment();
            return Instance.RunWorker(func);
        }

        private Task RunWorker(Func<Task<IDictionary<string, object>>> func)
        {
            var stack = new Stack(func);
            RegisterTask("User program code.", stack.Outputs.DataTask);
            return WhileRunning();
        }

        internal void RegisterTask(string description, Task task)
        {
            lock (_tasks)
            {
                _tasks.Enqueue((description, task));
            }
        }

        private async Task WhileRunning()
        {
            while (true)
            {
                string description;
                Task task;
                lock (_tasks)
                {
                    if (_tasks.Count == 0)
                    {
                        return;
                    }

                    (description, task) = _tasks.Dequeue(); 
                }

                Serilog.Log.Debug("Deployment awaiting: " + description);
                await task;
            }
        }
    }
}
