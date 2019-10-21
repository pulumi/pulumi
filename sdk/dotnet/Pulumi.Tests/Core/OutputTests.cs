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
        private static Output<T> CreateOutput<T>(T value, bool isKnown, bool isSecret = false)
            => new Output<T>(ImmutableHashSet<Resource>.Empty,
                Task.FromResult(new OutputData<T>(value, isKnown, isSecret)));

        private async Task Run(Func<Task> func, bool dryRun)
        {
            var originalValue = Deployment.DryRun;
            try
            {
                Deployment.DryRun = dryRun;
                await func().ConfigureAwait(false);
            }
            finally
            {
                Deployment.DryRun = originalValue;
            }
        }

        private Task RunInPreview(Func<Task> func)
            => Run(func, dryRun: true);

        private Task RunInNormal(Func<Task> func)
            => Run(func, dryRun: false);

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

        [Fact]
        public Task ApplyPreservesSecretOnKnownInPreview()
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
        public Task ApplyPreservesSecretEvenForUnknownInPreview()
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
}
