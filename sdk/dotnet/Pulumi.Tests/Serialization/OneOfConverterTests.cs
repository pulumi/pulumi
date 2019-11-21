// Copyright 2016-2019, Pulumi Corporation

using System;
using Google.Protobuf.WellKnownTypes;
using OneOf;
using Pulumi.Serialization;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class OneOfConverterTests : ConverterTests
    {
        [Fact]
        public void T0()
        {
            var data = Converter.ConvertValue<OneOf<int, string>>("", new Value { NumberValue = 1 });
            Assert.True(data.Value.IsT0);
            Assert.True(data.IsKnown);
            Assert.Equal(1, data.Value.AsT0);
        }

        [Fact]
        public void T1()
        {
            var data = Converter.ConvertValue<OneOf<int, string>>("", new Value { StringValue = "foo" });
            Assert.True(data.Value.IsT1);
            Assert.True(data.IsKnown);
            Assert.Equal("foo", data.Value.AsT1);
        }

        [Fact]
        public void WrongTypeThrows()
        {
            Assert.Throws<InvalidOperationException>(() =>
            {
                var data = Converter.ConvertValue<OneOf<int, string>>("", new Value { BoolValue = true });
            });
        }
    }
}
