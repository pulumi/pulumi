// Copyright 2016-2019, Pulumi Corporation

using System;
using System.ComponentModel;
using System.Text.Json;
using System.Threading.Tasks;
using Pulumi.Serialization;
using Xunit;

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
                Value = _value;
            }

            public static ContainerColor Red { get; } = new ContainerColor("red");
            public static ContainerColor Blue { get; } = new ContainerColor("blue");
            public static ContainerColor Yellow { get; } = new ContainerColor("yellow");

            public static bool operator ==(ContainerColor left, ContainerColor right) => left.Equals(right);
            public static bool operator !=(ContainerColor left, ContainerColor right) => !left.Equals(right);

            [EditorBrowsable(EditorBrowsableState.Never)]
            public override bool Equals(object? obj) => obj is ContainerColor other && Equals(other);
            public bool Equals(ContainerColor other) => string.Equals(_value, other._value, StringComparison.Ordinal);

            [EditorBrowsable(EditorBrowsableState.Never)]
            public override int GetHashCode() => _value?.GetHashCode() ?? 0;

            public override string ToString() => _value;
            public object Value { get; }
        }
        
        public enum ContainerSize
        {
            FourInch = 4,
            SixInch = 6,
            EightInch = 8,
        }
        
        public class ResourceArgs1 : ResourceArgs
        {
            [Input("size")]
            public Input<ContainerSize>? Size { get; set; }
            [Input("color")]
            public Input<ContainerColor>? Color { get; set; }
        }

        private async Task<JsonElement> Test(object args)
        {
            var serialized = await SerializeToValueAsync(args);
            var converted = Converter.ConvertValue<JsonElement>("", serialized);
            return converted.Value;
        }

        [Fact]
        public async Task Enums()
        {
            var args = new ResourceArgs1{
                Size = ContainerSize.SixInch,
                Color = ContainerColor.Blue
            };
            var value = await Test(args);
            var size = value.GetProperty("size").GetInt32();
            var color = value.GetProperty("color").GetString();
            Assert.Equal(6, size);
            Assert.Equal("blue", color);
        }
    }
}
