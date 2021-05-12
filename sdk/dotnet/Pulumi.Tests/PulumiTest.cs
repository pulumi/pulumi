// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Threading.Tasks;
using Moq;

namespace Pulumi.Tests
{
    public abstract class PulumiTest
    {
        private static Task Run(Action action, bool dryRun)
            => Run(() =>
            {
                action();
                return Task.CompletedTask;
            }, dryRun);

        private static async Task Run(Func<Task> func, bool dryRun)
        {
            var runner = new Mock<IRunner>(MockBehavior.Strict);
            runner.Setup(r => r.RegisterTask(It.IsAny<string>(), It.IsAny<Task>()));

            var mock = new Mock<IDeploymentInternal>(MockBehavior.Strict);
            mock.Setup(d => d.IsDryRun).Returns(dryRun);
            mock.Setup(d => d.Runner).Returns(runner.Object);

            Deployment.Instance = new DeploymentInstance(mock.Object);
            await func().ConfigureAwait(false);
        }

        protected static Task RunInPreview(Action action)
            => Run(action, dryRun: true);

        protected static Task RunInNormal(Action action)
            => Run(action, dryRun: false);

        protected static Task RunInPreview(Func<Task> func)
            => Run(func, dryRun: true);

        protected static Task RunInNormal(Func<Task> func)
            => Run(func, dryRun: false);
    }
}
