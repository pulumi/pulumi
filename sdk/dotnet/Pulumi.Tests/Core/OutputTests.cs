// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Xunit;

namespace Pulumi.Tests.Core
{
    public class OutputTests
    {
        private async Task RunInPreview(Func<Task> func)
        {
            var originalValue = Deployment.DryRun;
            try
            {
                Deployment.DryRun = true;
                await func().ConfigureAwait(false);
            }
            finally
            {
                Deployment.DryRun = originalValue;
            }
        }

        private static Output<T> CreateOutput<T>(T value, bool isKnown, bool isSecret = false)
            => new Output<T>(ImmutableHashSet<Resource>.Empty,
                Task.FromResult(new OutputData<T>(value, isKnown, isSecret)));

        [Fact]
        public Task ApplyCanRunOnKnownValueInPreview()
            => RunInPreview(async () =>
            {
                var o1 = CreateOutput(0, isKnown: true);
                var o2 = o1.Apply(a => a + 1);
                var data = await o2.DataTask.ConfigureAwait(false);
                Assert.True(data.IsKnown);
                Assert.Equal(1, data.Value);
            });

        [Fact]
        public Task ApplyProducesUnknownDefaultOnUnknownInPreview()
            => RunInPreview(async () =>
            {
                var o1 = CreateOutput(0, isKnown: false);
                var o2 = o1.Apply(a => a + 1);
                var data = await o2.DataTask.ConfigureAwait(false);
                Assert.False(data.IsKnown);
                Assert.Equal(0, data.Value);
            });
    }
}
