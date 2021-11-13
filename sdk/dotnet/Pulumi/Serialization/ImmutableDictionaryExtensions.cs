// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Immutable;

namespace Pulumi.Serialization
{
    internal static class ImmutableDictionaryExtensions
    {
        public static bool AnyValues<TKey, TValue>(
            this ImmutableDictionary<TKey, TValue> immutableDictionary,
            Func<TValue, bool> predicate)
            where TKey : notnull
        {
            foreach (var (_, val) in immutableDictionary)
            {
                if (predicate(val))
                {
                    return true;
                }
            }
            return false;
        }
    }
}
