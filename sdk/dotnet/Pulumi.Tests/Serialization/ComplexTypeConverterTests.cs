// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Pulumi.Serialization;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    [OutputType]
    public class ComplexType1
    {
        public readonly string S;
        public readonly bool B;
        public readonly int I;
        public readonly double D;
        public readonly ImmutableArray<bool> Array;
        public readonly ImmutableDictionary<string, int> Dict;

        [OutputConstructor]
        public ComplexType1(
            string s, bool b, int i, double d,
            ImmutableArray<bool> array, ImmutableDictionary<string, int> dict)
        {
            S = s;
            B = b;
            I = i;
            D = d;
            Array = array;
            Dict = dict;
        }
    }

    public class ComplexTypeConverterTests : ConverterTests
    {
        [Fact]
        public async Task ComplexType1()
        {
            var data = Converter.ConvertValue<ComplexType1>("", await SerializeToValueAsync(new Dictionary<string, object>
            {
                { "s", "str" },
                { "b", true },
                { "i", 42 },
                { "d", 1.5 },
                { "array", new List<object> { true, false } },
                { "dict", new Dictionary<object, object> { { "k", 10 } } },
            }));

            Assert.Equal("str", data.Value.S);
            Assert.Equal((object)true, data.Value.B);
            Assert.Equal(42, data.Value.I);
            Assert.Equal(1.5, data.Value.D);
            AssertEx.SequenceEqual(ImmutableArray<bool>.Empty.Add(true).Add(false), data.Value.Array);
            AssertEx.MapEqual(ImmutableDictionary<string, int>.Empty.Add("k", 10), data.Value.Dict);

            Assert.True(data.IsKnown);
        }
    }
}
