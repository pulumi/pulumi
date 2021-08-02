// Copyright 2016-2019, Pulumi Corporation

using System;
using OneOf;

// ReSharper disable PossiblyImpureMethodCallOnReadonlyVariable

namespace Pulumi
{
    /// <summary>
    /// Internal interface to allow our code to operate on <see cref="Union{T0, T1}"/>s in an
    /// untyped manner. Necessary as there is no reasonable way to write algorithms over
    /// heterogeneous instantiations of generic types.
    /// </summary>
    internal interface IUnion
    {
        object Value { get; }
    }

    /// <summary>
    /// Represents a <see href="https://en.wikipedia.org/wiki/Tagged_union">Tagged Union</see>.
    /// <para/>
    /// This is used to hold a value that could take on several different, but fixed, types. Only
    /// one of the types can be in use at any one time. It can be thought of as a type that has
    /// several "cases," each of which should be handled correctly when that type is manipulated.
    /// <para/>
    /// For example, a <see cref="Resource"/> property that could either store a <see cref="int"/>
    /// or a <see cref="string"/> can be represented as <c>Output&lt;int, string&gt;</c>.  The <see
    /// cref="Input{T}"/> version of this is <see cref="InputUnion{T0, T1}"/>.
    /// </summary>
    public readonly struct Union<T0, T1> : IEquatable<Union<T0, T1>>, IUnion
    {
        private readonly OneOf<T0, T1> _data;

        public T0 AsT0 => _data.AsT0;
        public T1 AsT1 => _data.AsT1;
        public bool IsT0 => _data.IsT0;
        public bool IsT1 => _data.IsT1;
        public object Value => _data.Value;

        private Union(OneOf<T0, T1> data)
            => _data = data;

        public static Union<T0, T1> FromT0(T0 input) => From(OneOf<T0, T1>.FromT0(input));
        public static Union<T0, T1> FromT1(T1 input) => From(OneOf<T0, T1>.FromT1(input));

        private static Union<X, Y> From<X, Y>(OneOf<X, Y> input) => new Union<X, Y>(input);

        public override bool Equals(object? obj) => obj is Union<T0, T1> union && Equals(union);
        public override int GetHashCode() => _data.GetHashCode();
        public override string ToString() => _data.ToString();

        public bool Equals(Union<T0, T1> union) => _data.Equals(union._data);

        public Union<TResult, T1> MapT0<TResult>(Func<T0, TResult> mapFunc) => From(_data.MapT0(mapFunc));
        public Union<T0, TResult> MapT1<TResult>(Func<T1, TResult> mapFunc) => From(_data.MapT1(mapFunc));

        public TResult Match<TResult>(Func<T0, TResult> f0, Func<T1, TResult> f1) => _data.Match(f0, f1);
        public void Switch(Action<T0> f0, Action<T1> f1) => _data.Switch(f0, f1);

        public bool TryPickT0(out T0 value, out T1 remainder) => _data.TryPickT0(out value, out remainder);
        public bool TryPickT1(out T1 value, out T0 remainder) => _data.TryPickT1(out value, out remainder);

        public static implicit operator Union<T0, T1>(T0 t) => FromT0(t);
        public static implicit operator Union<T0, T1>(T1 t) => FromT1(t);
    }
}
