// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;

namespace Pulumi
{
    internal static class Extensions
    {
        public static ImmutableArray<TResult> SelectAsArray<TItem, TResult>(this ImmutableArray<TItem> items, Func<TItem, TResult> map)
            => ImmutableArray.CreateRange(items, map);
    }
}
