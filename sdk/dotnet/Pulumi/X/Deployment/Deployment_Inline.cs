using System;
using System.Threading.Tasks;
using Pulumi.X.Automation;
using Pulumi.X.Automation.Runtime;

namespace Pulumi
{
    public partial class Deployment
    {
        internal Deployment(RuntimeSettings? settings)
        {
            if (settings is null)
                throw new ArgumentNullException(nameof(settings));

            _projectName = settings.Project;
            _stackName = settings.Stack;
            _isDryRun = settings.IsDryRun;

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

        internal static Task<int> RunInlineAsync(RuntimeSettings settings, PulumiFn func)
            => CreateInlineRunner(settings).RunAsync(() => Task.FromResult(func()), null);

        private static IRunner CreateInlineRunner(RuntimeSettings? settings)
        {
            // Serilog.Log.Logger = new LoggerConfiguration().MinimumLevel.Debug().WriteTo.Console().CreateLogger();

            Serilog.Log.Debug("Deployment.RunInline called.");
            lock (_instanceLock)
            {
                if (_instance != null)
                    throw new NotSupportedException("Deployment.Run can only be called a single time.");

                Serilog.Log.Debug("Creating new inline Deployment.");
                var deployment = new Deployment(settings);
                Instance = new DeploymentInstance(deployment);
                return deployment._runner;
            }
        }
    }
}
