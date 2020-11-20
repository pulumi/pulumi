using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;

namespace Pulumi.X.Automation.Commands
{
    internal interface IPulumiCmd
    {
        Task<CommandResult> RunAsync(
            IEnumerable<string> args,
            string workingDir,
            IDictionary<string, string> additionalEnv,
            Action<string>? onOutput = null,
            CancellationToken cancellationToken = default);
    }
}
