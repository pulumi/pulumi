// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Serialization;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class UnionConverterTests : ConverterTests
    {
        [Fact]
        public void T0()
        {
            var data = Converter.ConvertValue<Union<int, string>>("", new Value { NumberValue = 1 });
            Assert.True(data.Value.IsT0);
            Assert.True(data.IsKnown);
            Assert.Equal(1, data.Value.AsT0);
        }

        [Fact]
        public void T1()
        {
            var data = Converter.ConvertValue<Union<int, string>>("", new Value { StringValue = "foo" });
            Assert.True(data.Value.IsT1);
            Assert.True(data.IsKnown);
            Assert.Equal("foo", data.Value.AsT1);
        }

        [Fact]
        public async Task MixedList()
        {
            var data = Converter.ConvertValue<ImmutableArray<Union<int, string>>>("",
                await SerializeToValueAsync(new List<object> { 1, "foo" }));
            Assert.True(data.IsKnown);
            Assert.Equal(2, data.Value.Length);

            Assert.True(data.Value[0].IsT0);
            Assert.Equal(1, data.Value[0].AsT0);

            Assert.True(data.Value[1].IsT1);
            Assert.Equal("foo", data.Value[1].AsT1);
        }

        [Fact]
        public void WrongTypeThrows()
        {
            Assert.Throws<InvalidOperationException>(() =>
            {
                Converter.ConvertValue<Union<int, string>>("", new Value { BoolValue = true });
            });
        }
    }
}
