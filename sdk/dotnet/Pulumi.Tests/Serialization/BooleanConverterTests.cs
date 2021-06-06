// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Serialization;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class BooleanConverterTests : ConverterTests
    {
        [Fact]
        public void True()
        {
            var data = Converter.ConvertValue<bool>("", new Value { BoolValue = true });
            Assert.True(data.Value);
            Assert.True(data.IsKnown);
        }

        [Fact]
        public void False()
        {
            var data = Converter.ConvertValue<bool>("", new Value { BoolValue = false });

            Assert.False(data.Value);
            Assert.True(data.IsKnown);
        }

        [Fact]
        public void SecretTrue()
        {
            var data = Converter.ConvertValue<bool>("", CreateSecretValue(new Value { BoolValue = true }));

            Assert.True(data.Value);
            Assert.True(data.IsKnown);
            Assert.True(data.IsSecret);
        }

        [Fact]
        public void SecretFalse()
        {
            var data = Converter.ConvertValue<bool>("", CreateSecretValue(new Value { BoolValue = false }));

            Assert.False(data.Value);
            Assert.True(data.IsKnown);
            Assert.True(data.IsSecret);
        }

        [Fact]
        public void NonBooleanThrows()
        {
            Assert.Throws<InvalidOperationException>(() =>
            {
                Converter.ConvertValue<bool>("", new Value { StringValue = "" });
            });
        }

        [Fact]
        public Task NullInPreviewProducesFalseKnown()
        {
            return RunInPreview(() =>
            {
                var data = Converter.ConvertValue<bool>("", new Value { NullValue = NullValue.NullValue });

                Assert.False(data.Value);
                Assert.True(data.IsKnown);
            });
        }

        [Fact]
        public Task NullInNormalProducesFalseKnown()
        {
            return RunInNormal(() =>
            {
                var data = Converter.ConvertValue<bool>("", new Value { NullValue = NullValue.NullValue });

                Assert.False(data.Value);
                Assert.True(data.IsKnown);
            });
        }

        [Fact]
        public void UnknownProducesFalseUnknown()
        {
            var data = Converter.ConvertValue<bool>("", UnknownValue);

            Assert.False(data.Value);
            Assert.False(data.IsKnown);
        }

        [Fact]
        public void StringTest()
        {
            Assert.Throws<InvalidOperationException>(() =>
            {
                Converter.ConvertValue<bool>("", new Value { StringValue = "" });
            });
        }

        [Fact]
        public void NullableTrue()
        {
            var data = Converter.ConvertValue<bool?>("", new Value { BoolValue = true });
            Assert.True(data.Value);
            Assert.True(data.IsKnown);
        }

        [Fact]
        public void NullableFalse()
        {
            var data = Converter.ConvertValue<bool?>("", new Value { BoolValue = false });

            Assert.False(data.Value);
            Assert.True(data.IsKnown);
        }

        [Fact]
        public void NullableNull()
        {
            var data = Converter.ConvertValue<bool?>("", new Value { NullValue = NullValue.NullValue });

            Assert.Null(data.Value);
            Assert.True(data.IsKnown);
        }
    }
}
