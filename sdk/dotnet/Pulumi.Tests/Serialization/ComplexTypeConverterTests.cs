// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading.Tasks;
using Pulumi.Serialization;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class ComplexTypeConverterTests : ConverterTests
    {
        #region Simple case

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

        [Fact]
        public async Task TestComplexType1()
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

        #endregion

        #region Nested case

        [OutputType]
        public class ComplexType2
        {
            public readonly ComplexType1 C;
            public readonly ImmutableArray<ComplexType1> C2Array;
            public readonly ImmutableDictionary<string, ComplexType1> C2Map;

            [OutputConstructor]
            public ComplexType2(
                ComplexType1 c,
                ImmutableArray<ComplexType1> c2Array,
                ImmutableDictionary<string, ComplexType1> c2Map)
            {
                C = c;
                C2Array = c2Array;
                C2Map = c2Map;
            }
        }

        [Fact]
        public async Task TestComplexType2()
        {
            var data = Converter.ConvertValue<ComplexType2>("", await SerializeToValueAsync(new Dictionary<string, object>
            {
                {
                    "c", 
                    new Dictionary<string, object>
                    {
                        { "s", "str1" },
                        { "b", false },
                        { "i", 1 },
                        { "d", 1.1 },
                        { "array", new List<object> { false, false } },
                        { "dict", new Dictionary<object, object> { { "k", 1 } } },
                    }
                },
                {
                    "c2Array",
                    new List<object>
                    {
                        new Dictionary<string, object>
                        {
                            { "s", "str2" },
                            { "b", true },
                            { "i", 2 },
                            { "d", 2.2 },
                            { "array", new List<object> { false, true } },
                            { "dict", new Dictionary<object, object> { { "k", 2 } } },
                        }
                    }
                },
                { 
                    "c2Map",
                    new Dictionary<string, object>
                    {
                        {
                            "someKey",
                            new Dictionary<string, object>
                            {
                                { "s", "str3" },
                                { "b", false },
                                { "i", 3 },
                                { "d", 3.3 },
                                { "array", new List<object> { true, false } },
                                { "dict", new Dictionary<object, object> { { "k", 3 } } },
                            }
                        }
                    }
                }
            })).Value;

            var value = data.C;
            Assert.Equal("str1", value.S);
            Assert.Equal((object)false, value.B);
            Assert.Equal(1, value.I);
            Assert.Equal(1.1, value.D);
            AssertEx.SequenceEqual(ImmutableArray<bool>.Empty.Add(false).Add(false), value.Array);
            AssertEx.MapEqual(ImmutableDictionary<string, int>.Empty.Add("k", 1), value.Dict);

            Assert.Single(data.C2Array);
            value = data.C2Array[0];
            Assert.Equal("str2", value.S);
            Assert.Equal((object)true, value.B);
            Assert.Equal(2, value.I);
            Assert.Equal(2.2, value.D);
            AssertEx.SequenceEqual(ImmutableArray<bool>.Empty.Add(false).Add(true), value.Array);
            AssertEx.MapEqual(ImmutableDictionary<string, int>.Empty.Add("k", 2), value.Dict);

            Assert.Single(data.C2Map);
            var (key, val) = data.C2Map.Single();
            Assert.Equal("someKey", key);
            value = val;

            Assert.Equal("str3", value.S);
            Assert.Equal((object)false, value.B);
            Assert.Equal(3, value.I);
            Assert.Equal(3.3, value.D);
            AssertEx.SequenceEqual(ImmutableArray<bool>.Empty.Add(true).Add(false), value.Array);
            AssertEx.MapEqual(ImmutableDictionary<string, int>.Empty.Add("k", 3), value.Dict);
        }

        #endregion
    }
}
