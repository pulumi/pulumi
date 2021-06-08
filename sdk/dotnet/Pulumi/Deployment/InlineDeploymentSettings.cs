using System.Collections.Generic;
using Microsoft.Extensions.Logging;

namespace Pulumi
{
    internal class InlineDeploymentSettings
    {
        public string EngineAddr { get; }

        public string MonitorAddr { get; }

        public IDictionary<string, string> Config { get; }

        public IEnumerable<string>? ConfigSecretKeys { get; }

        public string Project { get; }

        public string Stack { get; }

        public int Parallel { get; }

        public bool IsDryRun { get; }
        
        public ILogger? Logger { get; }

        public InlineDeploymentSettings(
            string engineAddr,
            string monitorAddr,
            IDictionary<string, string> config,
            IEnumerable<string>? configSecretKeys,
            string project,
            string stack,
            int parallel,
            bool isDryRun,
            ILogger? logger)
        {
            EngineAddr = engineAddr;
            MonitorAddr = monitorAddr;
            Config = config;
            ConfigSecretKeys = configSecretKeys;
            Project = project;
            Stack = stack;
            Parallel = parallel;
            IsDryRun = isDryRun;
            Logger = logger;
        }
    }
}
