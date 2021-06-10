// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading;

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
        public InputList() : this(Output.Create(ImmutableArray<T>.Empty))
        {
        }

        private InputList(Output<ImmutableArray<T>> values)
            : base(values)
        {
        }

        public void Add(params Input<T>[] inputs)
        {
            // Make an Output from the values passed in, mix in with our own Output, and combine
            // both to produce the final array that we will now point at.
            _outputValue = Output.Concat(_outputValue, Output.All(inputs));
        }

        /// <summary>
        /// Concatenates the values in this list with the values in <paramref name="other"/>,
        /// returning the concatenated sequence in a new <see cref="InputList{T}"/>.
        /// </summary>
        public InputList<T> Concat(InputList<T> other)
            => Output.Concat(_outputValue, other._outputValue);

        internal InputList<T> Clone()
            => new InputList<T>(_outputValue);

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
            => Output.All(values);

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
            var data = await _outputValue.GetValueAsync().ConfigureAwait(false);
            foreach (var value in data)
            {
                yield return value;
            }
        }

        #endregion
    }
}
