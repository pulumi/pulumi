// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;

namespace Pulumi
{
    public class InputList<T> : IEnumerable
    {
        private Output<ImmutableArray<T>> _values;

        internal InputList() : this(Output.Create(ImmutableArray<T>.Empty))
        {
        }

        private InputList(Output<ImmutableArray<T>> values)
            => _values = values;

        public void Add(params Input<T>[] inputs)
        {
            var values1 = _values;
            var values2 = Output.All(inputs);
            _values = Output.All<ImmutableArray<T>>(values1, values2)
                            .Apply(a => a[0].AddRange(a[1]));
        }

        internal InputList<T> Clone()
            => new InputList<T>(_values);

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
            => values.Apply(a => ImmutableArray.CreateRange(a));

        public static implicit operator InputList<T>(Output<List<T>> values)
            => values.Apply(a => ImmutableArray.CreateRange(a));

        public static implicit operator InputList<T>(Output<ImmutableArray<T>> values)
            => new InputList<T>(values);

        #endregion

        #region IEnumerable

        IEnumerator IEnumerable.GetEnumerator()
            => throw new NotSupportedException("An InputList cannot be enumerated");

        #endregion
    }
}
