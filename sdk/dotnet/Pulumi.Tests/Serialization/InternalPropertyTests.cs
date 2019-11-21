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
    public class InternalPropertyTests : ConverterTests
    {
        [Fact]
        public void IgnoreInternalProperty()
        {
            var data = Converter.ConvertValue<ImmutableDictionary<string, string>>("", new Value
            {
                StructValue = new Struct
                {
                    Fields =
                    {
                        { "a", new Value { StringValue = "b" } },
                        { "__defaults", new Value { BoolValue = true } },
                    }
                }
            });
            Assert.True(data.IsKnown);
            Assert.True(data.Value.ContainsKey("a"));
            Assert.Equal("b", data.Value["a"]);
            Assert.False(data.Value.ContainsKey("__defaults"));
        }
    }
}
