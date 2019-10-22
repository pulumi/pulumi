// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System.Collections.Immutable;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
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
            source.SetResult(new Value { ListValue = new ListValue() });

            var data = await source.Output.DataTask;
            Assert.Equal(ImmutableArray<bool>.Empty, data.Value);
            Assert.True(data.IsKnown);
        }
    }
}
