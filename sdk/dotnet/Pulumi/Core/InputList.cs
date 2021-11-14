// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading;
using OneOf;

namespace Pulumi
{
    /// <summary>
    /// A list of values that can be passed in as the arguments to a <see cref="Resource"/>.
    /// The individual values are themselves <see cref="Input{T}"/>s.  i.e. the individual values
    /// can be concrete values or <see cref="Output{T}"/>s.
    /// <para/>
    /// <see cref="InputList{T}"/> differs from a normal <see cref="IList{T}"/> in that it is itself
    /// an <see cref="Input{T}"/>.  For example, a <see cref="Resource"/> that accepts an <see
    /// cref="InputList{T}"/> will accept not just a list but an <see cref="Output{T}"/>
    /// of a list.  This is important for cases where the <see cref="Output{T}"/>
    /// list from some <see cref="Resource"/> needs to be passed into another <see
    /// cref="Resource"/>.  Or for cases where creating the list invariably produces an <see
    /// cref="Output{T}"/> because its resultant value is dependent on other <see
    /// cref="Output{T}"/>s.
    /// <para/>
    /// This benefit of <see cref="InputList{T}"/> is also a limitation.  Because it represents a
    /// list of values that may eventually be created, there is no way to simply iterate over, or
    /// access the elements of the list synchronously.
    /// <para/>
    /// <see cref="InputList{T}"/> is designed to be easily used in object and collection
    /// initializers.  For example, a resource that accepts a list of inputs can be written in
    /// either of these forms:
    /// <para/>
    /// <code>
    ///     new SomeResource("name", new SomeResourceArgs {
    ///         ListProperty = { Value1, Value2, Value3 },
    ///     });
    /// </code>
    /// <para/>
    /// or
    /// <code>
    ///     new SomeResource("name", new SomeResourceArgs {
    ///         ListProperty = new [] { Value1, Value2, Value3 },
    ///     });
    /// </code>
    /// </summary>
    public sealed class InputList<T> : Input<ImmutableArray<T>>, IEnumerable, IAsyncEnumerable<Input<T>>
    {
        private OneOf<ImmutableArray<Input<T>>, Output<ImmutableArray<T>>> _value;

        public InputList() : this(ImmutableArray<Input<T>>.Empty)
        {
        }

        private InputList(ImmutableArray<Input<T>> values)
            : this(OneOf<ImmutableArray<Input<T>>, Output<ImmutableArray<T>>>.FromT0(values))
        {
        }

        private InputList(Output<ImmutableArray<T>> values)
            : this(OneOf<ImmutableArray<Input<T>>, Output<ImmutableArray<T>>>.FromT1(values))
        {
        }

        private InputList(OneOf<ImmutableArray<Input<T>>, Output<ImmutableArray<T>>> value)
            : base(ImmutableArray<T>.Empty)
        {
            _value = value;
        }

        private protected override Output<ImmutableArray<T>> ToOutput()
            => _value.Match(v => Output.All(v), v => v);

        private protected override object Value
            => _value.Value;

        public void Add(params Input<T>[] inputs)
        {
            if (_value.IsT0)
            {
                var combined = _value.AsT0.AddRange(inputs);
                _value = OneOf<ImmutableArray<Input<T>>, Output<ImmutableArray<T>>>.FromT0(combined);
            }
            else
            {
                // Make an Output from the values passed in, mix in with our own Output, and combine
                // both to produce the final array that we will now point at.
                var combined = Output.Concat(_value.AsT1, Output.All(inputs));
                _value = OneOf<ImmutableArray<Input<T>>, Output<ImmutableArray<T>>>.FromT1(combined);
            }
        }

        /// <summary>
        /// Concatenates the values in this list with the values in <paramref name="other"/>,
        /// returning the concatenated sequence in a new <see cref="InputList{T}"/>.
        /// </summary>
        public InputList<T> Concat(InputList<T> other)
        {
            if (_value.IsT0)
            {
                if (other._value.IsT0)
                {
                    return _value.AsT0.AddRange(other._value.AsT0);
                }
                else
                {
                    return Output.Concat(Output.All(_value.AsT0), other._value.AsT1);
                }
            }
            else
            {
                if (other._value.IsT0)
                {
                    return Output.Concat(_value.AsT1, Output.All(other._value.AsT0));
                }
                else
                {
                    return Output.Concat(_value.AsT1, other._value.AsT1);
                }
            }
        }

        internal InputList<T> Clone()
            => new InputList<T>(_value);

        #region construct from unary

        public static implicit operator InputList<T>(T value)
            => ImmutableArray.Create<Input<T>>(value);

        public static implicit operator InputList<T>(Output<T> value)
            => ImmutableArray.Create<Input<T>>(value);

        public static implicit operator InputList<T>(Input<T> value)
            => ImmutableArray.Create(value);

        #endregion

        #region construct from array

        public static implicit operator InputList<T>(T[] values)
            => ImmutableArray.CreateRange(values.Select(v => (Input<T>)v));

        public static implicit operator InputList<T>(Output<T>[] values)
            => ImmutableArray.CreateRange(values.Select(v => (Input<T>)v));

        public static implicit operator InputList<T>(Input<T>[] values)
            => ImmutableArray.CreateRange(values);

        #endregion

        #region construct from list

        public static implicit operator InputList<T>(List<T> values)
            => ImmutableArray.CreateRange(values);

        public static implicit operator InputList<T>(List<Output<T>> values)
            => ImmutableArray.CreateRange(values);

        public static implicit operator InputList<T>(List<Input<T>> values)
            => ImmutableArray.CreateRange(values);

        #endregion

        #region construct from immutable array

        public static implicit operator InputList<T>(ImmutableArray<T> values)
            => values.SelectAsArray(v => (Input<T>)v);

        public static implicit operator InputList<T>(ImmutableArray<Output<T>> values)
            => values.SelectAsArray(v => (Input<T>)v);

        public static implicit operator InputList<T>(ImmutableArray<Input<T>> values)
            => new InputList<T>(values);

        #endregion

        #region construct from Output of some list type.

        public static implicit operator InputList<T>(Output<T[]> values)
            => values.Apply(ImmutableArray.CreateRange);

        public static implicit operator InputList<T>(Output<List<T>> values)
            => values.Apply(ImmutableArray.CreateRange);

        public static implicit operator InputList<T>(Output<IEnumerable<T>> values)
            => values.Apply(ImmutableArray.CreateRange);

        public static implicit operator InputList<T>(Output<ImmutableArray<T>> values)
            => new InputList<T>(values);

        #endregion

        #region IEnumerable

        IEnumerator IEnumerable.GetEnumerator()
            => throw new NotSupportedException($"A {GetType().FullName} cannot be synchronously enumerated. Use {nameof(GetAsyncEnumerator)} instead.");

        public async IAsyncEnumerator<Input<T>> GetAsyncEnumerator(CancellationToken cancellationToken)
        {
            if (_value.IsT0)
            {
                foreach (var value in _value.AsT0)
                {
                    yield return value;
                }
            }
            else
            {
                var data = await _value.AsT1.GetValueAsync(whenUnknown: ImmutableArray<T>.Empty)
                    .ConfigureAwait(false);
                foreach (var value in data)
                {
                    yield return value;
                }
            }
        }

        #endregion
    }
}
