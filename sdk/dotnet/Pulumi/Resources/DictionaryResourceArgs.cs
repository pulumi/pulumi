using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi
{
    /// <summary>
    /// A special type of <see cref="ResourceArgs"/> with resource inputs represented
    /// as a loosely-typed dictionary of objects. Normally,
    /// <see cref="DictionaryResourceArgs"/> should not be used by resource providers
    /// since it's too low-level and provides low safety. Its target scenario are
    /// resources with a very dynamic shape of inputs.
    /// The input dictionary may only contain objects that are serializable by
    /// Pulumi, e.g. lists and dictionaries must be immutable.
    /// </summary>
    public class DictionaryResourceArgs : ResourceArgs
    {
        private readonly ImmutableDictionary<string, object?> _dictionary;
        
        /// <summary>
        /// Constructs an instance of <see cref="DictionaryResourceArgs"/> from
        /// a dictionary of input objects.
        /// </summary>
        /// <param name="dictionary">The input dictionary. It may only contain objects
        /// that are serializable by Pulumi, e.g. lists and dictionaries must be
        /// immutable.</param>
        public DictionaryResourceArgs(ImmutableDictionary<string, object?> dictionary)
        {
            _dictionary = dictionary;
        }

        internal override Task<ImmutableDictionary<string, object?>> ToDictionaryAsync()
            => Task.FromResult(_dictionary);
    }
}
