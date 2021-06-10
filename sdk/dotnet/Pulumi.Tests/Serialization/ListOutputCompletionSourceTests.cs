// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Pulumi.Serialization;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class ListConverterTests : ConverterTests
    {
        [Fact]
        public async Task EmptyList()
        {
            var data = Converter.ConvertValue<ImmutableArray<bool>>("", await SerializeToValueAsync(new List<bool>()));

            Assert.Equal(ImmutableArray<bool>.Empty, data.Value);
            Assert.True(data.IsKnown);
        }

        [Fact]
        public async Task ListWithElement()
        {
            var data = Converter.ConvertValue<ImmutableArray<bool>>("", await SerializeToValueAsync(new List<bool> { true }));

            AssertEx.SequenceEqual(ImmutableArray<bool>.Empty.Add(true), data.Value);
            Assert.True(data.IsKnown);
        }

        [Fact]
        public async Task SecretListWithElement()
        {
            var data = Converter.ConvertValue<ImmutableArray<bool>>("", await SerializeToValueAsync(Output.CreateSecret(new List<object> { true })));

            AssertEx.SequenceEqual(ImmutableArray<bool>.Empty.Add(true), data.Value);
            Assert.True(data.IsKnown);
            Assert.True(data.IsSecret);
        }

        [Fact]
        public async Task ListWithSecretElement()
        {
            var data = Converter.ConvertValue<ImmutableArray<bool>>("", await SerializeToValueAsync(new List<object> { Output.CreateSecret(true) }));

            AssertEx.SequenceEqual(ImmutableArray<bool>.Empty.Add(true), data.Value);
            Assert.True(data.IsKnown);
            Assert.True(data.IsSecret);
        }

        [Fact]
        public async Task ListWithUnknownElement()
        {
            var data = Converter.ConvertValue<ImmutableArray<bool>>("", await SerializeToValueAsync(new List<object> { Output<bool>.CreateUnknown(true) }));

            AssertEx.SequenceEqual(ImmutableArray<bool>.Empty.Add(false), data.Value);
            Assert.False(data.IsKnown);
            Assert.False(data.IsSecret);
        }
    }
}
