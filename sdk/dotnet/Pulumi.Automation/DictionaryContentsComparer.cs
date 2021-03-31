// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Generic;

namespace Pulumi.Automation
{
    /// Compares two dictionaries for equality by content, as F# maps would.
    internal sealed class DictionaryContentsComparer<K, V> : IEqualityComparer<IDictionary<K, V>> where K : notnull
    {
        private IEqualityComparer<K> _keyComparer;
        private IEqualityComparer<V> _valueComparer;

        public DictionaryContentsComparer(IEqualityComparer<K> keyComparer, IEqualityComparer<V> valueComparer)
        {
            this._keyComparer = keyComparer;
            this._valueComparer = valueComparer;
        }

        bool IEqualityComparer<IDictionary<K, V>>.Equals(IDictionary<K, V>? x, IDictionary<K, V>? y)
        {
            if (x == null)
            {
                return y == null;
            }
            if (y == null)
            {
                return x == null;
            }
            if (x.Count != y.Count)
            {
                return false;
            }
            var y2 = new Dictionary<K, V>(y, this._keyComparer);
            foreach (var pair in x)
            {
                if (!y2.ContainsKey(pair.Key))
                {
                    return false;
                }

                if (!this._valueComparer.Equals(pair.Value, y2[pair.Key]))
                {
                    return false;
                }
            }
            return true;
        }

        int IEqualityComparer<IDictionary<K, V>>.GetHashCode(IDictionary<K, V> obj)
        {
            return 0; // inefficient but correct
        }
    }
}
