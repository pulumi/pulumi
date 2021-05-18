﻿using System.Collections.Generic;

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

        public InlineDeploymentSettings(
            string engineAddr,
            string monitorAddr,
            IDictionary<string, string> config,
            string project,
            string stack,
            int parallel,
            bool isDryRun,
            IEnumerable<string>? configSecretKeys = null)
        {
            EngineAddr = engineAddr;
            MonitorAddr = monitorAddr;
            Config = config;
            Project = project;
            Stack = stack;
            Parallel = parallel;
            IsDryRun = isDryRun;
            ConfigSecretKeys = configSecretKeys;
        }
    }
}
