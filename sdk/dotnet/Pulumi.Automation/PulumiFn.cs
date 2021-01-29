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
    public sealed class PulumiFn
    {
        private readonly Func<CancellationToken, Task<IDictionary<string, object?>>> _program;

        private PulumiFn(Func<CancellationToken, Task<IDictionary<string, object?>>> program)
        {
            this._program = program;
        }

        internal Task<IDictionary<string, object?>> InvokeAsync(CancellationToken cancellationToken)
            => this._program(cancellationToken);

        /// <summary>
        /// Creates an asynchronous inline (in process) pulumi program.
        /// </summary>
        /// <param name="program">An asynchronous pulumi program that takes in a <see cref="CancellationToken"/> and returns an output.</param>
        public static PulumiFn Create(Func<CancellationToken, Task<IDictionary<string, object?>>> program)
            => new PulumiFn(program);

        /// <summary>
        /// Creates an asynchronous inline (in process) pulumi program.
        /// </summary>
        /// <param name="program">An asynchronous pulumi program that returns an output.</param>
        public static PulumiFn Create(Func<Task<IDictionary<string, object?>>> program)
            => new PulumiFn(cancellationToken => program());

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

            return new PulumiFn(wrapper);
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

            return new PulumiFn(wrapper);
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

            return new PulumiFn(wrapper);
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
            => Create(() => new TStack());
    }
}
