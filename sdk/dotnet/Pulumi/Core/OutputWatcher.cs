// Copyright 2016-2020, Pulumi Corporation

using System;
using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi
{
    /// <summary>
    /// Allows extracting some internal insights about an instance of
    /// <see cref="Output{T}"/>. 
    /// </summary>
    public sealed class OutputWatcher<T>
    {
        private readonly Output<T> _output;

        public OutputWatcher(Output<T> output)
        {
            _output = output;
            _output.DataTask.ContinueWith(t =>
            {
                var data = t.Result;
                var args = new OutputResolvedArgs<T>(data.Value, data.IsKnown, data.IsSecret);
                Resolved?.Invoke(this, args);
            });
        }

        /// <summary>
        /// Retrieve a set of resources that the output depends on.
        /// </summary>
        /// <returns></returns>
        public Task<ImmutableHashSet<Resource>> GetDependenciesAsync()
            => (_output as IOutput).GetResourcesAsync();

        /// <summary>
        /// Fires when the output value is resolved.
        /// </summary>
        public event EventHandler<OutputResolvedArgs<T>>? Resolved;
    }

    /// <summary>
    /// Arguments for <see cref="OutputWatcher{T}.Resolved"/> events handler.
    /// </summary>
    public sealed class OutputResolvedArgs<T> : EventArgs
    {
        internal OutputResolvedArgs(T value, bool isKnown, bool isSecret)
        {
            Value = value;
            IsKnown = isKnown;
            IsSecret = isSecret;
        }

        /// <summary>
        /// Output value, if known.
        /// </summary>
        public T Value { get; }

        /// <summary>
        /// Whether the output value is known.
        /// </summary>
        public bool IsKnown { get; }

        /// <summary>
        /// Whether the output value is a secret.
        /// </summary>
        public bool IsSecret { get; }
    }
}
