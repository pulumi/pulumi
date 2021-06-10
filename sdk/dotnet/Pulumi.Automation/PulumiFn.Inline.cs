// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Runtime.ExceptionServices;
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

        internal override async Task<ExceptionDispatchInfo?> InvokeAsync(IRunner runner, CancellationToken cancellationToken)
        {
            ExceptionDispatchInfo? info = null;

            await runner.RunAsync(async () =>
            {
                try
                {
                    return await this._program(cancellationToken).ConfigureAwait(false);
                }
                catch (Exception ex)
                {
                    info = ExceptionDispatchInfo.Capture(ex);
                    throw;
                }
            }, null).ConfigureAwait(false);

            return info;
        }
    }
}
