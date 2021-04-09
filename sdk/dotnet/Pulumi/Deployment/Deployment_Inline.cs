// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Runtime.ExceptionServices;
using System.Threading.Tasks;

namespace Pulumi
{
    public partial class Deployment
    {
        private Deployment(InlineDeploymentSettings settings)
        {
            if (settings is null)
                throw new ArgumentNullException(nameof(settings));

            _projectName = settings.Project;
            _stackName = settings.Stack;
            _isDryRun = settings.IsDryRun;
            SetAllConfig(settings.Config);

            if (string.IsNullOrEmpty(settings.MonitorAddr)
                || string.IsNullOrEmpty(settings.EngineAddr)
                || string.IsNullOrEmpty(_projectName)
                || string.IsNullOrEmpty(_stackName))
            {
                throw new InvalidOperationException("Inline execution was not provided the necessary parameters to run the Pulumi engine.");
            }

            Serilog.Log.Debug("Creating Deployment Engine.");
            Engine = new GrpcEngine(settings.EngineAddr);
            Serilog.Log.Debug("Created Deployment Engine.");

            Serilog.Log.Debug("Creating Deployment Monitor.");
            Monitor = new GrpcMonitor(settings.MonitorAddr);
            Serilog.Log.Debug("Created Deployment Monitor.");

            _runner = new Runner(this);
            _logger = new Logger(this, Engine);
        }

        internal static async Task<ExceptionDispatchInfo?> RunInlineAsync(InlineDeploymentSettings settings, Func<IRunner, Task<ExceptionDispatchInfo?>> func)
        {
            ExceptionDispatchInfo? exceptionDispatchInfo = null;

            await CreateRunnerAndRunAsync(
                () => new Deployment(settings),
                async runner =>
                {
                    exceptionDispatchInfo = await func(runner).ConfigureAwait(false);
                    return 1;
                })
                .ConfigureAwait(false);

            return exceptionDispatchInfo;
        }
    }
}
