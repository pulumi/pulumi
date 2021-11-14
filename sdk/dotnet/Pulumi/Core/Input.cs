// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Threading.Tasks;
using OneOf;

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
        object? Value { get; }
    }

    /// <summary>
    /// <see cref="Input{T}"/> is a property input for a <see cref="Resource"/>.  It may be a promptly
    /// available T, or the output from a existing <see cref="Resource"/>.
    /// </summary>
    public class Input<T> : IInput
    {
        private readonly OneOf<T, Output<T>> _value;

        private protected Input(T value)
            => _value = OneOf<T, Output<T>>.FromT0(value);

        private protected Input(Output<T> outputValue)
            => _value = OneOf<T, Output<T>>.FromT1(outputValue ?? throw new ArgumentNullException(nameof(outputValue)));

        private protected virtual Output<T> ToOutput()
            => _value.Match(v => Output.Create(v), v => v);

        private protected virtual object? Value
            => _value.Value;

        public static implicit operator Input<T>(T value)
            => new Input<T>(value);

        public static implicit operator Input<T>(Output<T> value)
            => new Input<T>(value);

        public static implicit operator Output<T>(Input<T> input)
            => input.ToOutput();

        IOutput IInput.ToOutput()
            => this.ToOutput();

        object? IInput.Value
            => this.Value;
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
