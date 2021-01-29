// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Runtime.ExceptionServices;
using System.Threading;
using System.Threading.Tasks;

namespace Pulumi.Automation
{
    internal class PulumiFn<TStack> : PulumiFn where TStack : Pulumi.Stack, new()
    {
        public PulumiFn()
        {
        }

        internal override async Task<ExceptionDispatchInfo?> InvokeAsync(IRunner runner, CancellationToken cancellationToken)
        {
            ExceptionDispatchInfo? info = null;

            await runner.RunAsync(() =>
            {
                try
                {
                    return new TStack();
                }
                catch (Exception ex)
                {
                    info = ExceptionDispatchInfo.Capture(ex);
                    throw;
                }
            }).ConfigureAwait(false);

            return info;
        }
    }
}
