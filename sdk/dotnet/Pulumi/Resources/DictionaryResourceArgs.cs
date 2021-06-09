using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Pulumi.Serialization;

namespace Pulumi
{
    /// <summary>
    /// A special type of <see cref="ResourceArgs"/> with resource inputs represented
    /// as a loosely-typed dictionary of objects. Normally,
    /// <see cref="DictionaryResourceArgs"/> should not be used by resource providers
    /// since it's too low-level and provides low safety. Its target scenario are
    /// resources with a very dynamic shape of inputs.
    /// The input dictionary may only contain objects that are serializable by
    /// Pulumi, i.e only the following types (or pulumi.Output of those types) are allowed:
    /// <list type="bullet">
    /// <item><description>Primitive types: <see cref="string"/>, <see cref="double"/>,
    /// <see cref="int"/>, <see cref="bool"/></description></item>
    /// <item><description><see cref="Asset"/>, <see cref="Archive"/>, or
    /// <see cref="AssetArchive"/></description></item>
    /// <item><description><see cref="System.Text.Json.JsonElement"/>
    /// </description></item>
    /// <item><description>Generic collections of the above:
    /// <see cref="ImmutableArray{T}"/>, <see cref="ImmutableDictionary{TKey,TValue}"/>
    /// with <see cref="string"/> keys, <see cref="Union{T0,T1}"/></description></item>
    /// </list>
    /// </summary>
    public sealed class DictionaryResourceArgs : ResourceArgs
    {
        private readonly ImmutableDictionary<string, object?> _dictionary;

        /// <summary>
        /// Constructs an instance of <see cref="DictionaryResourceArgs"/> from
        /// a dictionary of input objects.
        /// </summary>
        /// <param name="dictionary">The input dictionary. It may only contain objects
        /// that are serializable by Pulumi.</param>
        public DictionaryResourceArgs(ImmutableDictionary<string, object?> dictionary)
        {
            // Run a basic validation of types of values in the dictionary
            var seenTypes = new HashSet<Type>();
            foreach (var value in dictionary.Values)
            {
                if (value == null) continue;

                var targetType = value.GetType();
                if (value is IOutput)
                {
                    var type = value.GetType();
                    if (type.IsGenericType && type.GetGenericTypeDefinition() == typeof(Output<>))
                    {
                        targetType = type.GenericTypeArguments[0];
                    }
                }

                Converter.CheckTargetType(nameof(dictionary), targetType, seenTypes);
            }

            _dictionary = dictionary;
        }

        internal override Task<ImmutableDictionary<string, object?>> ToDictionaryAsync()
            => Task.FromResult(_dictionary);
    }
}
