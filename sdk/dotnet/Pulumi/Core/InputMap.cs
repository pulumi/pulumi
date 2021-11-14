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
    /// A mapping of <see cref="string"/>s to values that can be passed in as the arguments to a
    /// <see cref="Resource"/>. The individual values are themselves <see cref="Input{T}"/>s.  i.e.
    /// the individual values can be concrete values or <see cref="Output{T}"/>s.
    /// <para/>
    /// <see cref="InputMap{V}"/> differs from a normal <see cref="IDictionary{TKey,TValue}"/> in that it is
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
    public sealed class InputMap<V> : Input<ImmutableDictionary<string, V>>, IEnumerable, IAsyncEnumerable<Input<KeyValuePair<string, V>>>
    {
        private OneOf<ImmutableDictionary<string, Input<V>>, Output<ImmutableDictionary<string, V>>> _value;

        public InputMap() : this(ImmutableDictionary<string, Input<V>>.Empty)
        {
        }

        private InputMap(ImmutableDictionary<string, Input<V>> values)
            : this(OneOf<ImmutableDictionary<string, Input<V>>, Output<ImmutableDictionary<string, V>>>.FromT0(values))
        {
        }

        private InputMap(Output<ImmutableDictionary<string, V>> values)
            : this(OneOf<ImmutableDictionary<string, Input<V>>, Output<ImmutableDictionary<string, V>>>.FromT1(values))
        {
        }

        private InputMap(OneOf<ImmutableDictionary<string, Input<V>>, Output<ImmutableDictionary<string, V>>> value)
            : base(ImmutableDictionary<string, V>.Empty)
        {
            _value = value;
        }

        private protected override Output<ImmutableDictionary<string, V>> ToOutput()
            => _value.Match(
                v =>
                {
                    var kvps = v.ToImmutableArray();
                    var keys = kvps.SelectAsArray(kvp => kvp.Key);
                    var values = kvps.SelectAsArray(kvp => kvp.Value);
                    return Output.Tuple(Output.Create(keys), Output.All(values)).Apply(x =>
                    {
                        var builder = ImmutableDictionary.CreateBuilder<string, V>();
                        for (int i = 0; i < x.Item1.Length; i++)
                        {
                            builder.Add(x.Item1[i], x.Item2[i]);
                        }
                        return builder.ToImmutable();
                    });
                },
                v => v);

        private protected override object Value
            => _value.Value;

        public void Add(string key, Input<V> value)
        {

            if (_value.IsT0)
            {
                var combined = _value.AsT0.Add(key, value);
                _value = OneOf<ImmutableDictionary<string, Input<V>>, Output<ImmutableDictionary<string, V>>>.FromT0(combined);
            }
            else
            {
                var combined = Output.Tuple((Input<ImmutableDictionary<string, V>>)_value.AsT1, value)
                                     .Apply(x => x.Item1.Add(key, x.Item2));
                _value = OneOf<ImmutableDictionary<string, Input<V>>, Output<ImmutableDictionary<string, V>>>.FromT1(combined);
            }
        }

        public Input<V> this[string key]
        {
            set => Add(key, value);
        }

        /// <summary>
        /// Merge two instances of <see cref="InputMap{V}"/>. Returns a new <see cref="InputMap{V}"/>
        /// without modifying any of the arguments.
        /// <para/>If both maps contain the same key, the value from the second map takes over.
        /// </summary>
        /// <param name="first">The first <see cref="InputMap{V}"/>. Has lower priority in case of
        /// key clash.</param>
        /// <param name="second">The second <see cref="InputMap{V}"/>. Has higher priority in case of
        /// key clash.</param>
        /// <returns>A new instance of <see cref="InputMap{V}"/> that contains the items from
        /// both input maps.</returns>
        public static InputMap<V> Merge(InputMap<V> first, InputMap<V> second)
        {
            if (first._value.IsT0)
            {
                if (second._value.IsT0)
                {
                    var result = first._value.AsT0;
                    // Overwrite keys if duplicates are found
                    foreach (var (k, v) in second._value.AsT0)
                        result = result.SetItem(k, v);
                    return new InputMap<V>(result);
                }
                else
                {
                    return Output.Tuple(first.ToOutput(), second._value.AsT1)
                                 .Apply(dicts => Merge(dicts));
                }
            }
            else
            {
                if (second._value.IsT0)
                {
                    return Output.Tuple(first._value.AsT1, second.ToOutput())
                                 .Apply(dicts => Merge(dicts));
                }
                else
                {
                    return Output.Tuple(first._value.AsT1, second._value.AsT1)
                                 .Apply(dicts => Merge(dicts));
                }
            }

            Dictionary<string, V> Merge((ImmutableDictionary<string, V>, ImmutableDictionary<string, V>) dicts)
            {
                var result = new Dictionary<string, V>(dicts.Item1);
                // Overwrite keys if duplicates are found
                foreach (var (k, v) in dicts.Item2)
                    result[k] = v;
                return result;
            }
        }

        #region construct from dictionary types

        public static implicit operator InputMap<V>(Dictionary<string, V> values)
            => new InputMap<V>(ImmutableDictionary.CreateRange(values.Select(kvp => KeyValuePair.Create(kvp.Key, (Input<V>)kvp.Value))));

        public static implicit operator InputMap<V>(ImmutableDictionary<string, V> values)
            => new InputMap<V>(ImmutableDictionary.CreateRange(values.Select(kvp => KeyValuePair.Create(kvp.Key, (Input<V>)kvp.Value))));

        public static implicit operator InputMap<V>(Output<Dictionary<string, V>> values)
            => values.Apply(ImmutableDictionary.CreateRange);

        public static implicit operator InputMap<V>(Output<IDictionary<string, V>> values)
            => values.Apply(ImmutableDictionary.CreateRange);

        public static implicit operator InputMap<V>(Output<ImmutableDictionary<string, V>> values)
            => new InputMap<V>(values);

        #endregion

        #region IEnumerable

        IEnumerator IEnumerable.GetEnumerator()
            => throw new NotSupportedException($"A {GetType().FullName} cannot be synchronously enumerated. Use {nameof(GetAsyncEnumerator)} instead.");

        public async IAsyncEnumerator<Input<KeyValuePair<string, V>>> GetAsyncEnumerator(CancellationToken cancellationToken)
        {
            if (_value.IsT0)
            {
                foreach (var value in _value.AsT0)
                {
                    var input = (IInput)value.Value;
                    if (input.Value is IOutput)
                    {
                        yield return Output.Tuple((Input<string>)value.Key, value.Value)
                                           .Apply(x => KeyValuePair.Create(x.Item1, x.Item2));
                    }
                    else
                    {
                        yield return KeyValuePair.Create(value.Key, (V)input.Value!);
                    }
                }
            }
            else
            {
                var data = await _value.AsT1.GetValueAsync(whenUnknown: ImmutableDictionary<string, V>.Empty)
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
