// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Threading.Tasks;

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

        public static implicit operator Input<T>(T value)
            => Output.Create(value);

        public static implicit operator Input<T>(Output<T> value)
            => new Input<T>(value);

        public static implicit operator Output<T>(Input<T> input)
            => input._outputValue;

        IOutput IInput.ToOutput()
            => this.ToOutput();
    }

    public static class InputExtensions
    {
        /// <summary>
        /// <see cref="Output{T}.Apply{U}(Func{T, Output{U}?})"/> for more details.
        /// </summary>
        public static Output<U> Apply<T, U>(this Input<T>? input, Func<T, U> func)
            => input.ToOutput().Apply(func);

        /// <summary>
        /// <see cref="Output{T}.Apply{U}(Func{T, Output{U}?})"/> for more details.
        /// </summary>
        public static Output<U> Apply<T, U>(this Input<T>? input, Func<T, Task<U>> func)
            => input.ToOutput().Apply(func);

        /// <summary>
        /// <see cref="Output{T}.Apply{U}(Func{T, Output{U}?})"/> for more details.
        /// </summary>
        public static Output<U> Apply<T, U>(this Input<T>? input, Func<T, Input<U>?> func)
            => input.ToOutput().Apply(func);

        /// <summary>
        /// <see cref="Output{T}.Apply{U}(Func{T, Output{U}?})"/> for more details.
        /// </summary>
        public static Output<U> Apply<T, U>(this Input<T>? input, Func<T, Output<U>?> func)
            => input.ToOutput().Apply(func);

        public static Output<T> ToOutput<T>(this Input<T>? input)
            => input ?? Output.Create(default(T)!);
    }

    public static class InputListExtensions
    {
        public static void Add<T, U>(this InputList<Union<T, U>> list, Input<T> value)
            => list.Add(value.ToOutput().Apply(v => (Union<T, U>)v));

        public static void Add<T, U>(this InputList<Union<T, U>> list, Input<U> value)
            => list.Add(value.ToOutput().Apply(v => (Union<T, U>)v));
    }

    public static class InputMapExtensions
    {
        public static void Add<T, U>(this InputMap<Union<T, U>> map, string key, Input<T> value)
            => map.Add(key, value.ToOutput().Apply(v => (Union<T, U>)v));

        public static void Add<T, U>(this InputMap<Union<T, U>> map, string key, Input<U> value)
            => map.Add(key, value.ToOutput().Apply(v => (Union<T, U>)v));
    }
}
