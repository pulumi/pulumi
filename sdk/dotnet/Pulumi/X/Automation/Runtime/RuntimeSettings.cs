using System.Collections.Generic;

namespace Pulumi.X.Automation.Runtime
{
    internal class RuntimeSettings
    {
        public RuntimeSettings(string engineAddr, string monitorAddr, IDictionary<string, string> config, string project, string stack, int parallel, bool isDryRun)
        {
            EngineAddr = engineAddr;
            MonitorAddr = monitorAddr;
            Config = config;
            Project = project;
            Stack = stack;
            Parallel = parallel;
            IsDryRun = isDryRun;
        }

        public string EngineAddr { get; }
        public string MonitorAddr { get; }
        public IDictionary<string, string> Config { get; }
        public string Project { get; }
        public string Stack { get; }
        public int Parallel { get; }
        public bool IsDryRun { get; }
    }
}
