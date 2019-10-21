// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections;
using System.Collections.Generic;
using System.Collections.Immutable;

namespace Pulumi
{
    /// <summary>
    /// A mapping of <see cref="string"/>s to values that can be passed in as the arguments to a
    /// <see cref="Resource"/>. The individual values are themselves <see cref="Input{T}"/>s.  i.e.
    /// the individual values can be concrete values or <see cref="Output{T}"/>s.
    /// <para/>
    /// <see cref="InputMap{V}"/> differs from a normal <see cref="IDictionary{K,V}"/> in that it is
    /// itself an <see cref="Input{T}"/>.  For example, a <see cref="Resource"/> that accepts an
    /// <see cref="InputMap{V}"/> will accept not just a dictionary but an <see cref="Output{T}"/>
    /// of a dictionary as well.  This is important for cases where the <see cref="Output{T}"/>
    /// map from some <see cref="Resource"/> needs to be passed into another <see cref="Resource"/>.
    /// Or for cases where creating the map invariably produces an <see cref="Output{T}"/> because
    /// its resultant value is dependent on other <see cref="Output{T}"/>s.
    /// <para/>
    /// This benefit of <see cref="InputMap{V}"/> is also a limitation.  Because it represents a
    /// list of values that may eventually be created, there is no way to simply iterate over, or
    /// access the elements of the map synchronously.
    /// <para/>
    /// <see cref="InputMap{V}"/> is designed to be easily used in object and collection
    /// initializers.  For example, a resource that accepts a map of values can be written easily in
    /// this form:
    /// <para/>
    /// <code>
    ///     new SomeResource("name", new SomeResourceArgs {
    ///         MapProperty = {
    ///             { Key1, Value1 },
    ///             { Key2, Value2 },
    ///             { Key3, Value3 },
    ///         },
    ///     });
    /// </code>
    /// </summary>
    public class InputMap<V> : Input<ImmutableDictionary<string, V>>, IEnumerable
    {
        internal InputMap() : this(Output.Create(ImmutableDictionary<string, V>.Empty))
        {
        }

        private InputMap(Output<ImmutableDictionary<string, V>> values)
            : base(values)
        {
        }

        public void Add(string key, Input<V> value)
        {
            var inputDictionary = (Input<ImmutableDictionary<string, V>>)_outputValue;
            _outputValue = Output.Tuple(inputDictionary, value)
                                 .Apply(x => x.Item1.Add(key, x.Item2));
        }

        public Input<V> this[string key]
        {
            set => Add(key, value);
        }

        #region construct from dictionary types

        public static implicit operator InputMap<V>(Dictionary<string, V> values)
            => Output.Create(values);

        public static implicit operator InputMap<V>(ImmutableDictionary<string, V> values)
            => Output.Create(values);

        public static implicit operator InputMap<V>(Output<Dictionary<string, V>> values)
            => values.Apply(d => ImmutableDictionary.CreateRange(d));

        public static implicit operator InputMap<V>(Output<ImmutableDictionary<string, V>> values)
            => new InputMap<V>(values);

        #endregion

        #region IEnumerable

        IEnumerator IEnumerable.GetEnumerator()
            => throw new NotSupportedException("An InputMap cannot be enumerated");

        #endregion
    }
}
