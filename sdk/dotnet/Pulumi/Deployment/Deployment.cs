// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Threading.Tasks;
using Grpc.Core;
using Microsoft.Win32.SafeHandles;
using Pulumirpc;

namespace Pulumi
{
    /// <summary>
    /// <see cref="Deployment"/> is the entry-point to a Pulumi application. .NET applications
    /// should perform all startup logic they need in their <c>Main</c> method and then end with:
    /// <para>
    /// <c>
    /// static Task&lt;int&gt; Main(string[] args)
    /// {
    ///     // program initialization code ...
    ///     
    ///     return Deployment.Run(async () =>
    ///     {
    ///         // Code that creates resources.
    ///     });
    /// }
    /// </c>
    /// </para>
    /// Importantly: Cloud resources cannot be created outside of the lambda passed to any of the
    /// <see cref="Deployment.RunAsync(Action)"/> overloads.  Because cloud Resource construction is
    /// inherently asynchronous, the result of this function is a <see cref="Task{T}"/> which should
    /// then be returned or awaited.  This will ensure that any problems that are encountered during
    /// the running of the program are properly reported.  Failure to do this may lead to the
    /// program ending early before all resources are properly registered.
    /// </summary>
    public sealed partial class Deployment : IDeploymentInternal
    {
        private static IDeployment? _instance;

        /// <summary>
        /// The current running deployment instance. This is only available from inside the function
        /// passed to <see cref="Deployment.RunAsync(Action)"/> (or its overloads).
        /// </summary>
        public static IDeployment Instance
        {
            get => _instance ?? throw new InvalidOperationException("Trying to acquire Deployment.Instance before 'Run' was called.");
            internal set => _instance = (value ?? throw new ArgumentNullException(nameof(value)));
        }

        internal static IDeploymentInternal InternalInstance
            => (IDeploymentInternal)Instance;

        private readonly Options _options;
        private readonly string _projectName;
        private readonly string _stackName;
        private readonly bool _isDryRun;

        private readonly ILogger _logger;
        private readonly IRunner _runner;

        internal Engine.EngineClient Engine { get; }
        internal ResourceMonitor.ResourceMonitorClient Monitor { get; }

        internal Stack? _stack;
        internal Stack Stack
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

            if (string.IsNullOrEmpty(monitor) ||
                string.IsNullOrEmpty(engine) ||
                string.IsNullOrEmpty(project) ||
                string.IsNullOrEmpty(stack) ||
                !bool.TryParse(dryRun, out var dryRunValue) ||
                !bool.TryParse(queryMode, out var queryModeValue) ||
                !int.TryParse(parallel, out var parallelValue))
            {
                throw new InvalidOperationException("Program run without the Pulumi engine available; re-run using the `pulumi` CLI");
            }

            _isDryRun = dryRunValue;
            _stackName = stack;
            _projectName = project;

            _options = new Options(
                queryMode: queryModeValue, parallel: parallelValue, pwd: pwd,
                monitor: monitor, engine: engine, tracing: tracing);

            Serilog.Log.Debug("Creating Deployment Engine.");
            this.Engine = new Engine.EngineClient(new Channel(engine, ChannelCredentials.Insecure));
            Serilog.Log.Debug("Created Deployment Engine.");

            Serilog.Log.Debug("Creating Deployment Monitor.");
            this.Monitor = new ResourceMonitor.ResourceMonitorClient(new Channel(monitor, ChannelCredentials.Insecure));
            Serilog.Log.Debug("Created Deployment Monitor.");

            _runner = new Runner(this);
            _logger = new Logger(this, this.Engine);
        }

        string IDeployment.ProjectName => _projectName;
        string IDeployment.StackName => _stackName;
        bool IDeployment.IsDryRun => _isDryRun;

        Options IDeploymentInternal.Options => _options;
        ILogger IDeploymentInternal.Logger => _logger;
        IRunner IDeploymentInternal.Runner => _runner;

        Stack IDeploymentInternal.Stack
        {
            get => Stack;
            set => Stack = value;
        }
    }
}
