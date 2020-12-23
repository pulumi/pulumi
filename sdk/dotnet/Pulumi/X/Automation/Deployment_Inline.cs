using System;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Pulumi.X.Automation.Runtime;

namespace Pulumi
{
    public partial class Deployment
    {
        internal Deployment(RuntimeSettings? settings)
        {
            if (settings is null)
            {
                throw new ArgumentNullException(nameof(settings));
            }

            var monitor = settings.MonitorAddr;
            var engine = settings.EngineAddr;
            _projectName = settings.Project;
            _stackName = settings.Stack;
            _isDryRun = settings.IsDryRun;
            //var queryMode = Environment.GetEnvironmentVariable("PULUMI_QUERY_MODE");
            var parallel = Environment.GetEnvironmentVariable("PULUMI_PARALLEL");
            //var tracing = Environment.GetEnvironmentVariable("PULUMI_TRACING");

            if (string.IsNullOrEmpty(monitor) ||
                string.IsNullOrEmpty(engine) ||
                string.IsNullOrEmpty(_projectName) ||
                string.IsNullOrEmpty(_stackName))
            {
                throw new InvalidOperationException("Program run without the Pulumi engine available; re-run using the `pulumi` CLI");
            }

            Serilog.Log.Debug("Creating Deployment Engine.");
            Engine = new GrpcEngine(engine);
            Serilog.Log.Debug("Created Deployment Engine.");

            Serilog.Log.Debug("Creating Deployment Monitor.");
            Monitor = new GrpcMonitor(monitor);
            Serilog.Log.Debug("Created Deployment Monitor.");

            _runner = new Runner(this);
            _logger = new Logger(this, Engine);
        }

        internal Task<int> RunInstanceAsync(Action action)
            => RunAsync(() =>
            {
                action();
                return ImmutableDictionary<string, object?>.Empty;
            });
    }
}
