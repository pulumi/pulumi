// Copyright 2016-2021, Pulumi Corporation

using System.Collections.Generic;
using Microsoft.Extensions.Logging;

namespace Pulumi.Automation
{
    /// <summary>
    /// Options controlling the behavior of an <see cref="WorkspaceStack.UpAsync(UpOptions, System.Threading.CancellationToken)"/> operation.
    /// </summary>
    public sealed class UpOptions : UpdateOptions
    {
        public bool? ExpectNoChanges { get; set; }

        public bool? Diff { get; set; }

        public List<string>? Replace { get; set; }

        public bool? TargetDependents { get; set; }

        public PulumiFn? Program { get; set; }

        /// <summary>
        /// A custom logger instance that will be used for the action. Note that it will only be used
        /// if <see cref="Program"/> is also provided.
        /// </summary>
        public ILogger? Logger { get; set; }
    }
}
