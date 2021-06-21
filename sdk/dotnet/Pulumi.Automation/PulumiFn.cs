// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
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

        /// <summary>
        /// Invoke the appropriate run function on the <see cref="IRunner"/> instance. The exit code returned
        /// from the appropriate run function should be forwarded here as well.
        /// </summary>
        internal abstract Task<int> InvokeAsync(IRunner runner, CancellationToken cancellationToken);

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
            async Task<IDictionary<string, object?>> Wrapper(CancellationToken cancellationToken)
            {
                await program(cancellationToken).ConfigureAwait(false);
                return ImmutableDictionary<string, object?>.Empty;
            }

            return new PulumiFnInline(Wrapper);
        }

        /// <summary>
        /// Creates an asynchronous inline (in process) pulumi program.
        /// </summary>
        /// <param name="program">An asynchronous pulumi program.</param>
        public static PulumiFn Create(Func<Task> program)
        {
            async Task<IDictionary<string, object?>> Wrapper(CancellationToken cancellationToken)
            {
                await program().ConfigureAwait(false);
                return ImmutableDictionary<string, object?>.Empty;
            }

            return new PulumiFnInline(Wrapper);
        }

        /// <summary>
        /// Creates an inline (in process) pulumi program.
        /// </summary>
        /// <param name="program">A pulumi program that returns an output.</param>
        public static PulumiFn Create(Func<IDictionary<string, object?>> program)
        {
            Task<IDictionary<string, object?>> Wrapper(CancellationToken cancellationToken)
            {
                var output = program();
                return Task.FromResult(output);
            }

            return new PulumiFnInline(Wrapper);
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
            where TStack : Stack, new()
            => new PulumiFn<TStack>(() => new TStack());

        /// <summary>
        /// Creates an inline (in process) pulumi program via a traditional <see cref="Pulumi.Stack"/> implementation.
        /// <para/>
        ///     When invoked, a new stack instance will be resolved based
        ///     on the provided <typeparamref name="TStack"/> type parameter
        ///     using the <paramref name="serviceProvider"/>.
        /// </summary>
        /// <typeparam name="TStack">The <see cref="Pulumi.Stack"/> type.</typeparam>
        /// <param name="serviceProvider">The service provider that will be used to resolve an instance of <typeparamref name="TStack"/>.</param>
        public static PulumiFn Create<TStack>(IServiceProvider serviceProvider)
            where TStack : Stack
            => new PulumiFnServiceProvider(serviceProvider, typeof(TStack));

        /// <summary>
        /// Creates an inline (in process) pulumi program via a traditional <see cref="Pulumi.Stack"/> implementation.
        /// <para/>
        ///     When invoked, a new stack instance will be resolved based
        ///     on the provided <paramref name="stackType"/> type parameter
        ///     using the <paramref name="serviceProvider"/>.
        /// </summary>
        /// <param name="serviceProvider">The service provider that will be used to resolve an instance of type <paramref name="stackType"/>.</param>
        /// <param name="stackType">The stack type, which must derive from <see cref="Pulumi.Stack"/>.</param>
        public static PulumiFn Create(IServiceProvider serviceProvider, Type stackType)
            => new PulumiFnServiceProvider(serviceProvider, stackType);
    }
}
