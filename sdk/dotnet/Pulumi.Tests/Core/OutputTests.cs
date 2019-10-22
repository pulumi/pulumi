// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System.Collections.Immutable;
using System.Threading.Tasks;
using Pulumi.Serialization;
using Xunit;

namespace Pulumi.Tests.Core
{
    public partial class OutputTests : PulumiTest
    {
        private static Output<T> CreateOutput<T>(T value, bool isKnown, bool isSecret = false)
            => new Output<T>(ImmutableHashSet<Resource>.Empty,
                Task.FromResult(OutputData.Create(value, isKnown, isSecret)));

        public class PreviewTests
        {
            [Fact]
            public Task ApplyCanRunOnKnownValue()
                => RunInPreview(async () =>
                {
                    var o1 = CreateOutput(0, isKnown: true);
                    var o2 = o1.Apply(a => a + 1);
                    var data = await o2.DataTask.ConfigureAwait(false);
                    Assert.True(data.IsKnown);
                    Assert.Equal(1, data.Value);
                });

            [Fact]
            public Task ApplyProducesUnknownDefaultOnUnknown()
                => RunInPreview(async () =>
                {
                    var o1 = CreateOutput(0, isKnown: false);
                    var o2 = o1.Apply(a => a + 1);
                    var data = await o2.DataTask.ConfigureAwait(false);
                    Assert.False(data.IsKnown);
                    Assert.Equal(0, data.Value);
                });

            [Fact]
            public Task ApplyPreservesSecretOnKnown()
                => RunInPreview(async () =>
                {
                    var o1 = CreateOutput(0, isKnown: true, isSecret: true);
                    var o2 = o1.Apply(a => a + 1);
                    var data = await o2.DataTask.ConfigureAwait(false);
                    Assert.True(data.IsKnown);
                    Assert.True(data.IsSecret);
                    Assert.Equal(1, data.Value);
                });

            [Fact]
            public Task ApplyPreservesSecretOnUnknown()
                => RunInPreview(async () =>
                {
                    var o1 = CreateOutput(0, isKnown: false, isSecret: true);
                    var o2 = o1.Apply(a => a + 1);
                    var data = await o2.DataTask.ConfigureAwait(false);
                    Assert.False(data.IsKnown);
                    Assert.True(data.IsSecret);
                    Assert.Equal(0, data.Value);
                });
        }

        public class NormalTests
        {
            [Fact]
            public Task ApplyCanRunOnKnownValue()
                => RunInNormal(async () =>
                {
                    var o1 = CreateOutput(0, isKnown: true);
                    var o2 = o1.Apply(a => a + 1);
                    var data = await o2.DataTask.ConfigureAwait(false);
                    Assert.True(data.IsKnown);
                    Assert.Equal(1, data.Value);
                });

            [Fact]
            public Task ApplyProducesKnownOnUnknown()
                => RunInNormal(async () =>
                {
                    var o1 = CreateOutput(0, isKnown: false);
                    var o2 = o1.Apply(a => a + 1);
                    var data = await o2.DataTask.ConfigureAwait(false);
                    Assert.False(data.IsKnown);
                    Assert.Equal(1, data.Value);
                });

            [Fact]
            public Task ApplyPreservesSecretOnKnown()
                => RunInNormal(async () =>
                {
                    var o1 = CreateOutput(0, isKnown: true, isSecret: true);
                    var o2 = o1.Apply(a => a + 1);
                    var data = await o2.DataTask.ConfigureAwait(false);
                    Assert.True(data.IsKnown);
                    Assert.True(data.IsSecret);
                    Assert.Equal(1, data.Value);
                });

            [Fact]
            public Task ApplyPreservesSecretOnUnknown()
                => RunInNormal(async () =>
                {
                    var o1 = CreateOutput(0, isKnown: false, isSecret: true);
                    var o2 = o1.Apply(a => a + 1);
                    var data = await o2.DataTask.ConfigureAwait(false);
                    Assert.False(data.IsKnown);
                    Assert.True(data.IsSecret);
                    Assert.Equal(1, data.Value);
                });
        }
    }
}
