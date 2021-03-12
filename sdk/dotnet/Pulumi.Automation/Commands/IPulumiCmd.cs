// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;

namespace Pulumi.Automation.Commands
{
    internal interface IPulumiCmd
    {
        Task<CommandResult> RunAsync(
            IEnumerable<string> args,
            string workingDir,
            IDictionary<string, string> additionalEnv,
            Action<string>? onStandardOutput = null,
            Action<string>? onStandardError = null,
            CancellationToken cancellationToken = default);
    }
}
