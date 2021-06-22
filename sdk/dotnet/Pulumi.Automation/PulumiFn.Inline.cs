// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;

namespace Pulumi.Automation
{
    internal sealed class PulumiFnInline : PulumiFn
    {
        private readonly Func<CancellationToken, Task<IDictionary<string, object?>>> _program;

        public PulumiFnInline(Func<CancellationToken, Task<IDictionary<string, object?>>> program)
        {
            this._program = program;
        }

        /// <inheritdoc/>
        internal override Task<int> InvokeAsync(IRunner runner, CancellationToken cancellationToken)
            => runner.RunAsync(() => this._program(cancellationToken), null);
    }
}
