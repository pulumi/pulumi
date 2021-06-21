// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Threading;
using System.Threading.Tasks;

namespace Pulumi.Automation
{
    internal class PulumiFn<TStack> : PulumiFn where TStack : Stack
    {
        private readonly Func<TStack> _stackFactory;

        public PulumiFn(Func<TStack> stackFactory)
        {
            this._stackFactory = stackFactory;
        }

        /// <inheritdoc/>
        internal override Task<int> InvokeAsync(IRunner runner, CancellationToken cancellationToken)
            => runner.RunAsync(this._stackFactory);
    }
}
