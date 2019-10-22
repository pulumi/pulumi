// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using System.Threading.Tasks;
using Moq;

namespace Pulumi.Tests
{
    public abstract class PulumiTest
    {
        private static async Task Run(Func<Task> func, bool dryRun)
        {
            var mock = new Mock<IDeployment>(MockBehavior.Strict);
            mock.Setup(d => d.IsDryRun).Returns(dryRun);

            Deployment.Instance = mock.Object;
            await func().ConfigureAwait(false);
        }

        protected static Task RunInPreview(Func<Task> func)
            => Run(func, dryRun: true);

        protected static Task RunInNormal(Func<Task> func)
            => Run(func, dryRun: false);
    }
}
