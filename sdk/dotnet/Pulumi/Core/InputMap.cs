// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections;
using System.Collections.Generic;
using System.Collections.Immutable;

namespace Pulumi
{
    public class InputMap<V> : IEnumerable, IInput
    {
        private Output<ImmutableDictionary<string, V>> _values;

        internal InputMap() : this(Output.Create(ImmutableDictionary<string, V>.Empty))
        {
        }

        private InputMap(Output<ImmutableDictionary<string, V>> values)
            => _values = values;

        internal Output<ImmutableDictionary<string, V>> GetInnerMap()
            => _values;

        IOutput IInput.ToOutput()
            => _values;

        public void Add(string key, Input<V> value)
        {
            var inputDictionary = (Input<ImmutableDictionary<string, V>>)_values;
            _values = Output.Tuple(inputDictionary, value)
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
