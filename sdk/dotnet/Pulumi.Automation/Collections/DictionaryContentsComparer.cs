// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Generic;

namespace Pulumi.Automation.Collections
{
    /// Compares two dictionaries for equality by content, as F# maps would.
    internal sealed class DictionaryContentsComparer<TKey, TValue> : IEqualityComparer<IDictionary<TKey, TValue>> where TKey : notnull
    {
        private readonly IEqualityComparer<TKey> _keyComparer;
        private readonly IEqualityComparer<TValue> _valueComparer;

        public DictionaryContentsComparer(IEqualityComparer<TKey> keyComparer, IEqualityComparer<TValue> valueComparer)
        {
            this._keyComparer = keyComparer;
            this._valueComparer = valueComparer;
        }

        bool IEqualityComparer<IDictionary<TKey, TValue>>.Equals(IDictionary<TKey, TValue>? x, IDictionary<TKey, TValue>? y)
        {
            if (x == null)
            {
                return y == null;
            }
            if (y == null)
            {
                return false;
            }
            if (ReferenceEquals(x, y))
            {
                return true;
            }
            if (x.Count != y.Count)
            {
                return false;
            }
            var y2 = new Dictionary<TKey, TValue>(y, this._keyComparer);
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

        int IEqualityComparer<IDictionary<TKey, TValue>>.GetHashCode(IDictionary<TKey, TValue> obj)
        {
            return 0; // inefficient but correct
        }
    }
}
