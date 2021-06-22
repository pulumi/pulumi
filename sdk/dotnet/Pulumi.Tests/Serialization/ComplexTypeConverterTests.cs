// Copyright 2016-2019, Pulumi Corporation

using System;
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
        [EnumType]
        public readonly struct ContainerColor : IEquatable<ContainerColor>
        {
            private readonly string _value;

            private ContainerColor(string value)
            {
                _value = value ?? throw new ArgumentNullException(nameof(value));
            }

            public static ContainerColor Red { get; } = new ContainerColor("red");
            public static ContainerColor Blue { get; } = new ContainerColor("blue");
            public static ContainerColor Yellow { get; } = new ContainerColor("yellow");

            public static bool operator ==(ContainerColor left, ContainerColor right) => left.Equals(right);
            public static bool operator !=(ContainerColor left, ContainerColor right) => !left.Equals(right);

            public static explicit operator string(ContainerColor value) => value._value;

            public override bool Equals(object? obj) => obj is ContainerColor other && Equals(other);
            public bool Equals(ContainerColor other) => string.Equals(_value, other._value, StringComparison.Ordinal);

            public override int GetHashCode() => _value.GetHashCode();

            public override string ToString() => _value;
        }

        public enum ContainerSize
        {
            FourInch = 4,
            SixInch = 6,
            EightInch = 8,
        }

        [OutputType]
        public class ComplexType1
        {
            public readonly string S;
            public readonly bool B;
            public readonly int I;
            public readonly double D;
            public readonly ImmutableArray<bool> Array;
            public readonly ImmutableDictionary<string, int> Dict;
            public readonly object Obj;
            public readonly ContainerSize Size;
            public readonly ContainerColor Color;

            [OutputConstructor]
            public ComplexType1(
                string s, bool b, int i, double d,
                ImmutableArray<bool> array, ImmutableDictionary<string, int> dict, object obj,
                ContainerSize size, ContainerColor color)
            {
                S = s;
                B = b;
                I = i;
                D = d;
                Array = array;
                Dict = dict;
                Obj = obj;
                Size = size;
                Color = color;
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
                { "obj", "test" },
                { "size", 6 },
                { "color", "blue" },
            }));

            Assert.Equal("str", data.Value.S);
            Assert.Equal((object)true, data.Value.B);
            Assert.Equal(42, data.Value.I);
            Assert.Equal(1.5, data.Value.D);
            AssertEx.SequenceEqual(ImmutableArray<bool>.Empty.Add(true).Add(false), data.Value.Array);
            AssertEx.MapEqual(ImmutableDictionary<string, int>.Empty.Add("k", 10), data.Value.Dict);
            Assert.Equal("test", data.Value.Obj);
            Assert.Equal(ContainerSize.SixInch, data.Value.Size);
            Assert.Equal(ContainerColor.Blue, data.Value.Color);

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
                        { "obj", 50.0 },
                        { "size", 8 },
                        { "color", "red" },
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
                            { "obj", true },
                            { "size", 4 },
                            { "color", "yellow" },
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
                                { "obj", new Dictionary<object, object> { { "o", 5.5 } } },
                                { "size", 6 },
                                { "color", "blue" },
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
            Assert.Equal(50.0, value.Obj);
            Assert.Equal(ContainerSize.EightInch, value.Size);
            Assert.Equal(ContainerColor.Red, value.Color);

            Assert.Single(data.C2Array);
            value = data.C2Array[0];
            Assert.Equal("str2", value.S);
            Assert.Equal((object)true, value.B);
            Assert.Equal(2, value.I);
            Assert.Equal(2.2, value.D);
            AssertEx.SequenceEqual(ImmutableArray<bool>.Empty.Add(false).Add(true), value.Array);
            AssertEx.MapEqual(ImmutableDictionary<string, int>.Empty.Add("k", 2), value.Dict);
            Assert.Equal(true, value.Obj);
            Assert.Equal(ContainerSize.FourInch, value.Size);
            Assert.Equal(ContainerColor.Yellow, value.Color);

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
            AssertEx.MapEqual(ImmutableDictionary<string, object>.Empty.Add("o", 5.5), (IDictionary<string, object>)value.Obj);
            Assert.Equal(ContainerSize.SixInch, value.Size);
            Assert.Equal(ContainerColor.Blue, value.Color);
        }

        #endregion
    }
}
