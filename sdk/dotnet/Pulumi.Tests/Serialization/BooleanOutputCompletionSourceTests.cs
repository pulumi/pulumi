// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Serialization;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class BooleanOutputCompletionSourceTests : CompletionSourceTests
    {
        [Fact]
        public async Task True()
        {
            var source = new OutputCompletionSource<bool>(resource: null);
            source.SetValue("", new Value { BoolValue = true });

            var data = await source.Output.DataTask;
            Assert.True(data.Value);
            Assert.True(data.IsKnown);
        }

        [Fact]
        public async Task False()
        {
            var source = new OutputCompletionSource<bool>(resource: null);
            source.SetValue("", new Value { BoolValue = false });

            var data = await source.Output.DataTask;
            Assert.False(data.Value);
            Assert.True(data.IsKnown);
        }
        [Fact]
        public async Task SecretTrue()
        {
            var source = new OutputCompletionSource<bool>(resource: null);
            source.SetValue("", CreateSecretValue(new Value { BoolValue = true }));

            var data = await source.Output.DataTask;
            Assert.True(data.Value);
            Assert.True(data.IsKnown);
            Assert.True(data.IsSecret);
        }

        [Fact]
        public async Task SecretFalse()
        {
            var source = new OutputCompletionSource<bool>(resource: null);
            source.SetValue("", CreateSecretValue(new Value { BoolValue = false }));

            var data = await source.Output.DataTask;
            Assert.False(data.Value);
            Assert.True(data.IsKnown);
            Assert.True(data.IsSecret);
        }

        [Fact]
        public void NonBooleanThrows()
        {
            Assert.Throws<InvalidOperationException>(() =>
            {
                var source = new OutputCompletionSource<bool>(resource: null);
                source.SetValue("", new Value { StringValue = "" });
            });
        }

        [Fact]
        public Task NullInPreviewProducesFalseKnown()
        {
            return RunInPreview(async () =>
            {
                var source = new OutputCompletionSource<bool>(resource: null);
                source.SetValue("", new Value { NullValue = NullValue.NullValue });

                var data = await source.Output.DataTask;
                Assert.False(data.Value);
                Assert.True(data.IsKnown);
            });
        }

        [Fact]
        public Task NullInNormalProducesFalseKnown()
        {
            return RunInNormal(async () =>
            {
                var source = new OutputCompletionSource<bool>(resource: null);
                source.SetValue("", new Value { NullValue = NullValue.NullValue });

                var data = await source.Output.DataTask;
                Assert.False(data.Value);
                Assert.True(data.IsKnown);
            });
        }

        [Fact]
        public async Task UnknownProducesFalseUnknown()
        {
            var source = new OutputCompletionSource<bool>(resource: null);
            source.SetValue("", UnknownValue);

            var data = await source.Output.DataTask;
            Assert.False(data.Value);
            Assert.False(data.IsKnown);
        }

        [Fact]
        public void StringTest()
        {
            Assert.Throws<InvalidOperationException>(() =>
            {
                var source = new OutputCompletionSource<bool>(resource: null);
                source.SetValue("", new Value { StringValue = "" });
            });
        }
    }
}
