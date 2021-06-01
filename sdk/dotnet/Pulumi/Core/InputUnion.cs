// Copyright 2016-2019, Pulumi Corporation

namespace Pulumi
{
    /// <summary>
    /// Represents an <see cref="Input{T}"/> value that can be one of two different types. For
    /// example, it might potentially be an <see cref="int"/> some of the time or a <see
    /// cref="string"/> in other cases.
    /// </summary>
    public sealed class InputUnion<T0, T1> : Input<Union<T0, T1>>
    {
        public InputUnion() : this(Output.Create(default(Union<T0, T1>)))
        {
        }

        private InputUnion(Output<Union<T0, T1>> oneOf)
            : base(oneOf)
        {
        }

        #region common conversions

        public static implicit operator InputUnion<T0, T1>(T0 value)
            => Output.Create(value);

        public static implicit operator InputUnion<T0, T1>(T1 value)
            => Output.Create(value);

        public static implicit operator InputUnion<T0, T1>(Output<T0> value)
            => new InputUnion<T0, T1>(value.Apply(Union<T0, T1>.FromT0));

        public static implicit operator InputUnion<T0, T1>(Output<T1> value)
            => new InputUnion<T0, T1>(value.Apply(Union<T0, T1>.FromT1));

        #endregion
    }
}
