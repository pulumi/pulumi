// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using System.Diagnostics.CodeAnalysis;

namespace Pulumi
{
    /// <summary>
    /// Internal interface to allow our code to operate on inputs in an untyped manner. Necessary as
    /// there is no reasonable way to write algorithms over heterogeneous instantiations of generic
    /// types.
    /// </summary>
    internal interface IInput
    {
        IOutput ToOutput();
    }

    /// <summary>
    /// <see cref="Input{T}"/> is a property input for a <see cref="Resource"/>.  It may be a promptly
    /// available T, or the output from a existing <see cref="Resource"/>.
    /// </summary>
    public class Input<T> : IInput
    {
        /// <summary>
        /// Technically, in .net we can represent Inputs entirely using the Output type (since
        /// Outputs can wrap values and promises).  However, it would look very weird to state that
        /// the inputs to a resource *had* to be Outputs. So we basically just come up with this
        /// wrapper type so things look sensible, even though under the covers we implement things
        /// using the exact same type
        /// </summary>
        private protected Output<T> _outputValue;

        private protected Input(Output<T> outputValue)
            => _outputValue = outputValue ?? throw new ArgumentNullException(nameof(outputValue));

        public static implicit operator Input<T>([MaybeNull]T value)
            => Output.Create(value);

        public static implicit operator Input<T>(Output<T> value)
            => new Input<T>(value);

        public static implicit operator Output<T>(Input<T> input)
            => input._outputValue;

        public Output<T> ToOutput()
            => this;

        IOutput IInput.ToOutput()
            => ToOutput();
    }
}
