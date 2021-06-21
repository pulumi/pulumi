// Copyright 2016-2021, Pulumi Corporation

using System;
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

        /// <inheritdoc/>
        internal override Task<int> InvokeAsync(IRunner runner, CancellationToken cancellationToken)
            => runner.RunAsync(() =>
            {
                if (this._serviceProvider is null)
                    throw new ArgumentNullException(nameof(this._serviceProvider), $"The provided service provider was null by the time this {nameof(PulumiFn)} was invoked.");

                return this._serviceProvider.GetService(this._stackType) as Stack
                    ?? throw new ApplicationException(
                        $"Failed to resolve instance of type {this._stackType.FullName} from service provider. Register the type with the service provider before this {nameof(PulumiFn)} is invoked.");
            });
    }
}
