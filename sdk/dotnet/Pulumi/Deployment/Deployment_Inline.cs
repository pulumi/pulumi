using System;
using System.Collections.Generic;
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

            //var queryMode = Environment.GetEnvironmentVariable("PULUMI_QUERY_MODE");
            //var parallel = Environment.GetEnvironmentVariable("PULUMI_PARALLEL");
            //var tracing = Environment.GetEnvironmentVariable("PULUMI_TRACING");

            if (string.IsNullOrEmpty(settings.MonitorAddr)
                || string.IsNullOrEmpty(settings.EngineAddr)
                || string.IsNullOrEmpty(_projectName)
                || string.IsNullOrEmpty(_stackName))
            {
                throw new InvalidOperationException("Program run without the Pulumi engine available; re-run using the `pulumi` CLI");
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

        internal static Task<int> RunInlineAsync(InlineDeploymentSettings settings, Func<Task<IDictionary<string, object?>>> func)
            => CreateRunner(() => new Deployment(settings)).RunAsync(func, null);
    }
}
