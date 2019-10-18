// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections;
using System.Collections.Generic;
using System.Collections.Immutable;

namespace Pulumi
{
    public class InputMap<K, V> : IEnumerable
    {
        private Output<ImmutableDictionary<K, V>> _values;

        internal InputMap() : this(Output.Create(ImmutableDictionary<K, V>.Empty))
        {
        }

        private InputMap(Output<ImmutableDictionary<K, V>> values)
            => _values = values;

        internal Output<ImmutableDictionary<K, V>> GetInnerMap()
            => _values;

        public void Add(Input<K> key, Input<V> value)
        {
            var inputDictionary = (Input<ImmutableDictionary<K, V>>)_values;
            _values = Output.Tuple(inputDictionary, key, value)
                            .Apply(x => x.Item1.Add(x.Item2, x.Item3));
        }

        public Input<V> this[Input<K> key]
        {
            set => Add(key, value);
        }

        #region construct from dictionary types

        public static implicit operator InputMap<K, V>(Dictionary<K, V> values)
            => Output.Create(values);

        public static implicit operator InputMap<K, V>(ImmutableDictionary<K, V> values)
            => Output.Create(values);

        public static implicit operator InputMap<K, V>(Output<Dictionary<K, V>> values)
            => values.Apply(d => ImmutableDictionary.CreateRange(d));

        public static implicit operator InputMap<K, V>(Output<ImmutableDictionary<K, V>> values)
            => new InputMap<K, V>(values);

        #endregion

        #region IEnumerable

        IEnumerator IEnumerable.GetEnumerator()
            => throw new NotSupportedException("An InputMap cannot be enumerated");

        #endregion
    }
}
