// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Reflection;
using System.Runtime.ExceptionServices;
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

        internal override async Task<ExceptionDispatchInfo?> InvokeAsync(IRunner runner, CancellationToken cancellationToken)
        {
            ExceptionDispatchInfo? info = null;

            await runner.RunAsync(() =>
            {
                try
                {
                    return this._stackFactory();
                }
                // because we are newing a generic, reflection comes in to
                // construct the instance. And if there is an exception in
                // the constructor of the user-provided TStack, it will be wrapped
                // in TargetInvocationException - which is not the exception
                // we want to throw to the consumer.
                catch (TargetInvocationException ex) when (ex.InnerException != null)
                {
                    info = ExceptionDispatchInfo.Capture(ex.InnerException);
                    throw;
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
