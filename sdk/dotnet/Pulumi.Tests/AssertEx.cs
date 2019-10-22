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
    }
}
