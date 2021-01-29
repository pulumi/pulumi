// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Runtime.ExceptionServices;
using System.Threading;
using System.Threading.Tasks;

namespace Pulumi.Automation
{
    /// <summary>
    /// A Pulumi program as an inline function (in process).
    /// </summary>
    public abstract class PulumiFn
    {
        internal PulumiFn()
        {
        }

        internal abstract Task<ExceptionDispatchInfo?> InvokeAsync(IRunner runner, CancellationToken cancellationToken);

        /// <summary>
        /// Creates an asynchronous inline (in process) pulumi program.
        /// </summary>
        /// <param name="program">An asynchronous pulumi program that takes in a <see cref="CancellationToken"/> and returns an output.</param>
        public static PulumiFn Create(Func<CancellationToken, Task<IDictionary<string, object?>>> program)
            => new PulumiFnInline(program);

        /// <summary>
        /// Creates an asynchronous inline (in process) pulumi program.
        /// </summary>
        /// <param name="program">An asynchronous pulumi program that returns an output.</param>
        public static PulumiFn Create(Func<Task<IDictionary<string, object?>>> program)
            => new PulumiFnInline(cancellationToken => program());

        /// <summary>
        /// Creates an asynchronous inline (in process) pulumi program.
        /// </summary>
        /// <param name="program">An asynchronous pulumi program that takes in a <see cref="CancellationToken"/>.</param>
        public static PulumiFn Create(Func<CancellationToken, Task> program)
        {
            Func<CancellationToken, Task<IDictionary<string, object?>>> wrapper = async cancellationToken =>
            {
                await program(cancellationToken).ConfigureAwait(false);
                return ImmutableDictionary<string, object?>.Empty;
            };

            return new PulumiFnInline(wrapper);
        }

        /// <summary>
        /// Creates an asynchronous inline (in process) pulumi program.
        /// </summary>
        /// <param name="program">An asynchronous pulumi program.</param>
        public static PulumiFn Create(Func<Task> program)
        {
            Func<CancellationToken, Task<IDictionary<string, object?>>> wrapper = async cancellationToken =>
            {
                await program().ConfigureAwait(false);
                return ImmutableDictionary<string, object?>.Empty;
            };

            return new PulumiFnInline(wrapper);
        }

        /// <summary>
        /// Creates an inline (in process) pulumi program.
        /// </summary>
        /// <param name="program">A pulumi program that returns an output.</param>
        public static PulumiFn Create(Func<IDictionary<string, object?>> program)
        {
            Func<CancellationToken, Task<IDictionary<string, object?>>> wrapper = cancellationToken =>
            {
                var output = program();
                return Task.FromResult(output);
            };

            return new PulumiFnInline(wrapper);
        }

        /// <summary>
        /// Creates an inline (in process) pulumi program.
        /// </summary>
        /// <param name="program">A pulumi program.</param>
        public static PulumiFn Create(Action program)
            => Create(() => { program(); return ImmutableDictionary<string, object?>.Empty; });

        /// <summary>
        /// Creates an inline (in process) pulumi program via a traditional <see cref="Pulumi.Stack"/> implementation.
        /// </summary>
        /// <typeparam name="TStack">The <see cref="Pulumi.Stack"/> type.</typeparam>
        public static PulumiFn Create<TStack>()
            where TStack : Pulumi.Stack, new()
            => new PulumiFn<TStack>();
    }
}
