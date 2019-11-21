// Copyright 2016-2018, Pulumi Corporation

using OneOf;

namespace Pulumi.Core
{
    /// <summary>
    /// Represents an <see cref="Input{T}"/> value that can be one of two different types. For
    /// example, it might potentially be an <see cref="int"/> some of the time or a <see
    /// cref="string"/> in other cases.
    /// </summary>
    public sealed class InputOneOf<T0, T1> : Input<OneOf<T0, T1>>
    {
        public InputOneOf() : this(Output.Create(default(OneOf<T0, T1>)))
        {
        }

        private InputOneOf(Output<OneOf<T0, T1>> oneOf)
            : base(oneOf)
        {
        }

        #region common conversions

        public static implicit operator InputOneOf<T0, T1>(T0 value)
            => Output.Create(value);

        public static implicit operator InputOneOf<T0, T1>(T1 value)
            => Output.Create(value);

        public static implicit operator InputOneOf<T0, T1>(Output<T0> value)
            => new InputOneOf<T0, T1>(value.Apply(v => OneOf<T0, T1>.FromT0(v)));

        public static implicit operator InputOneOf<T0, T1>(Output<T1> value)
            => new InputOneOf<T0, T1>(value.Apply(v => OneOf<T0, T1>.FromT1(v)));

        #endregion
    }
}
