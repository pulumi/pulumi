// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System.Collections.Generic;
using Xunit;

namespace Pulumi.Tests
{
    public static class AssertEx
    {
        public static void SequenceEqual<T>(IEnumerable<T> expected, IEnumerable<T> actual)
            => Assert.Equal(expected, actual);

        public static void MapEqual<TKey, TValue>(IDictionary<TKey, TValue> expected, IDictionary<TKey, TValue> actual) where TKey : notnull
        {
            Assert.Equal(expected.Count, actual.Count);
            foreach (var (key, actualValue) in actual)
            {
                Assert.True(expected.TryGetValue(key, out var expectedValue));
                Assert.Equal(expectedValue, actualValue);
            }
        }
    }
}
