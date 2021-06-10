// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Reflection;
using System.Runtime.ExceptionServices;
using System.Threading;
using System.Threading.Tasks;

namespace Pulumi.Automation
{
    internal class PulumiFnServiceProvider : PulumiFn
    {
        private readonly IServiceProvider _serviceProvider;
        private readonly Type _stackType;

        public PulumiFnServiceProvider(
            IServiceProvider serviceProvider,
            Type stackType)
        {
            if (serviceProvider is null)
                throw new ArgumentNullException(nameof(serviceProvider));

            if (stackType is null)
                throw new ArgumentNullException(nameof(stackType));

            var pulumiStackType = typeof(Stack);
            if (!pulumiStackType.IsAssignableFrom(stackType) || pulumiStackType == stackType)
                throw new ArgumentException($"Provided stack type must derive from {pulumiStackType.FullName}.", nameof(stackType));

            this._serviceProvider = serviceProvider;
            this._stackType = stackType;
        }

        internal override async Task<ExceptionDispatchInfo?> InvokeAsync(IRunner runner, CancellationToken cancellationToken)
        {
            ExceptionDispatchInfo? info = null;

            await runner.RunAsync(() =>
            {
                try
                {
                    if (this._serviceProvider is null)
                        throw new ArgumentNullException(nameof(this._serviceProvider), $"The provided service provider was null by the time this {nameof(PulumiFn)} was invoked.");

                    return this._serviceProvider.GetService(this._stackType) as Stack
                        ?? throw new ApplicationException(
                            $"Failed to resolve instance of type {this._stackType.FullName} from service provider. Register the type with the service provider before this {nameof(PulumiFn)} is invoked.");
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
