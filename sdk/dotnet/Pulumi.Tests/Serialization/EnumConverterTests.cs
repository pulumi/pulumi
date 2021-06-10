// Copyright 2016-2020, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Globalization;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Serialization;
using Xunit;
using Type = System.Type;

namespace Pulumi.Tests.Serialization
{
    public class EnumConverterTests : ConverterTests
    {
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

            [EditorBrowsable(EditorBrowsableState.Never)]
            public override bool Equals(object? obj) => obj is ContainerColor other && Equals(other);
            public bool Equals(ContainerColor other) => string.Equals(_value, other._value, StringComparison.Ordinal);

            [EditorBrowsable(EditorBrowsableState.Never)]
            public override int GetHashCode() => _value.GetHashCode();

            public override string ToString() => _value;
        }

        [EnumType]
        public readonly struct ContainerBrightness : IEquatable<ContainerBrightness>
        {
            private readonly double _value;

            private ContainerBrightness(double value)
            {
                _value = value;
            }

            public static ContainerBrightness One { get; } = new ContainerBrightness(1.0);
            public static ContainerBrightness ZeroPointOne { get; } = new ContainerBrightness(0.1);

            public static bool operator ==(ContainerBrightness left, ContainerBrightness right) => left.Equals(right);
            public static bool operator !=(ContainerBrightness left, ContainerBrightness right) => !left.Equals(right);

            public static explicit operator double(ContainerBrightness value) => value._value;

            [EditorBrowsable(EditorBrowsableState.Never)]
            public override bool Equals(object? obj) => obj is ContainerBrightness other && Equals(other);
            // ReSharper disable once CompareOfFloatsByEqualityOperator
            public bool Equals(ContainerBrightness other) => _value == other._value;

            [EditorBrowsable(EditorBrowsableState.Never)]
            public override int GetHashCode() => _value.GetHashCode();

            public override string ToString() => _value.ToString(CultureInfo.InvariantCulture);
        }

        public enum ContainerSize
        {
            FourInch = 4,
            SixInch = 6,
            EightInch = 8,
        }

        public static IEnumerable<object[]> StringEnums()
            => new[]
            {
                new object[] { ContainerColor.Red },
                new object[] { ContainerColor.Blue },
                new object[] { ContainerColor.Yellow },
            };

        [Theory]
        [MemberData(nameof(StringEnums))]
        public async Task StringEnum(ContainerColor input)
        {
            var data = Converter.ConvertValue<ContainerColor>("", await SerializeToValueAsync(input));

            Assert.Equal(input, data.Value);
            Assert.True(data.IsKnown);
        }

        public static IEnumerable<object[]> DoubleEnums()
            => new[]
            {
                new object[] { ContainerBrightness.One },
                new object[] { ContainerBrightness.ZeroPointOne },
            };

        [Theory]
        [MemberData(nameof(DoubleEnums))]
        public async Task DoubleEnum(ContainerBrightness input)
        {
            var data = Converter.ConvertValue<ContainerBrightness>("", await SerializeToValueAsync(input));

            Assert.Equal(input, data.Value);
            Assert.True(data.IsKnown);
        }

        [Theory]
        [InlineData(ContainerSize.FourInch)]
        [InlineData(ContainerSize.SixInch)]
        [InlineData(ContainerSize.EightInch)]
        [InlineData((ContainerSize)(-1))]
        [InlineData((ContainerSize)int.MinValue)]
        [InlineData((ContainerSize)int.MaxValue)]
        public async Task Int32Enum(ContainerSize input)
        {
            var data = Converter.ConvertValue<ContainerSize>("", await SerializeToValueAsync(input));

            Assert.Equal(input, data.Value);
            Assert.True(data.IsKnown);
        }

        [EnumType]
        public readonly struct Unserializable1
        {
        }

        [EnumType]
        public readonly struct Unserializable2
        {
            private Unserializable2(string value)
            {
            }
        }

        public static IEnumerable<object[]> UnserializableEnums()
            => new[]
            {
                new object[] { new Unserializable1() },
                new object[] { new Unserializable2() },
            };

        [Theory]
        [MemberData(nameof(UnserializableEnums))]
        public async Task SerializingUnserializableEnumsThrows(object input)
        {
            await Assert.ThrowsAsync<InvalidOperationException>(async () =>
            {
                await SerializeToValueAsync(input);
            });
        }

        [EnumType]
        public readonly struct Unconvertible1
        {
            private Unconvertible1(string value)
            {
            }
            public static explicit operator double(Unconvertible1 value) => default;
        }

        [EnumType]
        public readonly struct Unconvertible2
        {
            private Unconvertible2(double value)
            {
            }
            public static explicit operator string(Unconvertible2 value) => "";
        }

        public static IEnumerable<object[]> UnconvertibleEnums()
            => new[]
            {
                new object[] { typeof(Unserializable1) },
                new object[] { typeof(Unserializable2) },
                new object[] { typeof(Unconvertible1) },
                new object[] { typeof(Unconvertible2) },
            };

        [Theory]
        [MemberData(nameof(UnconvertibleEnums))]
        public void CheckingUnconvertibleEnumsThrows(Type targetType)
        {
            Assert.Throws<InvalidOperationException>(() =>
            {
                var seenTypes = new HashSet<Type>();
                Converter.CheckTargetType("", targetType, seenTypes);
            });
        }

        public static IEnumerable<object[]> EnumsWithUnconvertibleValues()
            => new[]
            {
                new object[] { typeof(ContainerColor), new Value { NumberValue = 1.0 } },
                new object[] { typeof(ContainerBrightness), new Value { StringValue = "hello" } },
                new object[] { typeof(ContainerSize), new Value { StringValue = "hello" } },
            };

        [Theory]
        [MemberData(nameof(EnumsWithUnconvertibleValues))]
        public void ConvertingUnconvertibleValuesThrows(Type targetType, Value value)
        {
            Assert.Throws<InvalidOperationException>(() =>
            {
                Converter.ConvertValue("", value, targetType);
            });
        }
    }
}
