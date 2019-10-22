// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Pulumi.Serialization;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class ListOutputCompletionSourceTests : CompletionSourceTests
    {
        [Fact]
        public async Task EmptyList()
        {
            var source = new ListOutputCompletionSource<bool>(resource: null, Deserializers.BoolDeserializer);
            source.SetResult(await SerializeToValueAsync(new List<bool>()));

            var data = await source.Output.DataTask;
            Assert.Equal(ImmutableArray<bool>.Empty, data.Value);
            Assert.True(data.IsKnown);
        }

        [Fact]
        public async Task ListWithElement()
        {
            var source = new ListOutputCompletionSource<bool>(resource: null, Deserializers.BoolDeserializer);
            source.SetResult(await SerializeToValueAsync(new List<bool> { true }));

            var data = await source.Output.DataTask;
            AssertEx.SequenceEqual(ImmutableArray<bool>.Empty.Add(true), data.Value);
            Assert.True(data.IsKnown);
        }

        [Fact]
        public async Task SecretListWithElement()
        {
            var source = new ListOutputCompletionSource<bool>(resource: null, Deserializers.BoolDeserializer);
            source.SetResult(await SerializeToValueAsync(Output.CreateSecret(new List<object> { true })));

            var data = await source.Output.DataTask;
            AssertEx.SequenceEqual(ImmutableArray<bool>.Empty.Add(true), data.Value);
            Assert.True(data.IsKnown);
            Assert.True(data.IsSecret);
        }

        [Fact]
        public async Task ListWithSecretElement()
        {
            var source = new ListOutputCompletionSource<bool>(resource: null, Deserializers.BoolDeserializer);
            source.SetResult(await SerializeToValueAsync(new List<object> { Output.CreateSecret(true) }));

            var data = await source.Output.DataTask;
            AssertEx.SequenceEqual(ImmutableArray<bool>.Empty.Add(true), data.Value);
            Assert.True(data.IsKnown);
            Assert.True(data.IsSecret);
        }

        [Fact]
        public async Task SecretListWithUnknownElement()
        {
            var source = new ListOutputCompletionSource<bool>(resource: null, Deserializers.BoolDeserializer);
            source.SetResult(await SerializeToValueAsync(new List<object> { CreateUnknownOutput(true) }));

            var data = await source.Output.DataTask;
            AssertEx.SequenceEqual(ImmutableArray<bool>.Empty.Add(true), data.Value);
            Assert.False(data.IsKnown);
            Assert.True(data.IsSecret);
        }
    }
}
