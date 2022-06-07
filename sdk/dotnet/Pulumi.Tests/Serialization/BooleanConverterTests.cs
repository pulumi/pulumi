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
            var data = Converter.ConvertValue<bool>(NoWarn, "", new Value { BoolValue = true });
            Assert.True(data.Value);
            Assert.True(data.IsKnown);
        }

        [Fact]
        public void False()
        {
            var data = Converter.ConvertValue<bool>(NoWarn, "", new Value { BoolValue = false });

            Assert.False(data.Value);
            Assert.True(data.IsKnown);
        }

        [Fact]
        public void SecretTrue()
        {
            var data = Converter.ConvertValue<bool>(NoWarn, "", CreateSecretValue(new Value { BoolValue = true }));

            Assert.True(data.Value);
            Assert.True(data.IsKnown);
            Assert.True(data.IsSecret);
        }

        [Fact]
        public void SecretFalse()
        {
            var data = Converter.ConvertValue<bool>(NoWarn, "", CreateSecretValue(new Value { BoolValue = false }));

            Assert.False(data.Value);
            Assert.True(data.IsKnown);
            Assert.True(data.IsSecret);
        }

        [Fact]
        public void NonBooleanLogs()
        {
            string? loggedError = null;
            Action<string> warn = error => loggedError = error;
            var data = Converter.ConvertValue<bool>(warn, "", new Value { StringValue = "" });

            Assert.False(data.Value);
            Assert.True(data.IsKnown);

            Assert.Equal("Expected System.Boolean but got System.String deserializing ", loggedError);
        }

        [Fact]
        public Task NullInPreviewProducesFalseKnown()
        {
            return RunInPreview(() =>
            {
                var data = Converter.ConvertValue<bool>(NoWarn, "", new Value { NullValue = NullValue.NullValue });

                Assert.False(data.Value);
                Assert.True(data.IsKnown);
            });
        }

        [Fact]
        public Task NullInNormalProducesFalseKnown()
        {
            return RunInNormal(() =>
            {
                var data = Converter.ConvertValue<bool>(NoWarn, "", new Value { NullValue = NullValue.NullValue });

                Assert.False(data.Value);
                Assert.True(data.IsKnown);
            });
        }

        [Fact]
        public void UnknownProducesFalseUnknown()
        {
            var data = Converter.ConvertValue<bool>(NoWarn, "", UnknownValue);

            Assert.False(data.Value);
            Assert.False(data.IsKnown);
        }

        [Fact]
        public void NullableTrue()
        {
            var data = Converter.ConvertValue<bool?>(NoWarn, "", new Value { BoolValue = true });
            Assert.True(data.Value);
            Assert.True(data.IsKnown);
        }

        [Fact]
        public void NullableFalse()
        {
            var data = Converter.ConvertValue<bool?>(NoWarn, "", new Value { BoolValue = false });

            Assert.False(data.Value);
            Assert.True(data.IsKnown);
        }

        [Fact]
        public void NullableNull()
        {
            var data = Converter.ConvertValue<bool?>(NoWarn, "", new Value { NullValue = NullValue.NullValue });

            Assert.Null(data.Value);
            Assert.True(data.IsKnown);
        }
    }
}
