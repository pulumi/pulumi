using System.Threading.Tasks;
using Pulumi.Testing;
using Pulumi.Tests.Mocks;
using Xunit;

namespace Pulumi.Tests
{
    public class DeploymentTests
    {
        [Fact]
        public async Task DeploymentInstancesAreSeparate()
        {
            DeploymentInstance? instanceOne = null;
            DeploymentInstance? instanceTwo = null;

            var tcs = new TaskCompletionSource<int>();
            var runTaskOne = Deployment.CreateRunnerAndRunAsync(
                () => new Deployment(new MockEngine(), new MockMonitor(new MyMocks()), null),
                runner =>
                {
                    instanceOne = Deployment.Instance;
                    return tcs.Task;
                });

            await Deployment.CreateRunnerAndRunAsync(
                () => new Deployment(new MockEngine(), new MockMonitor(new MyMocks()), null),
                runner =>
                {
                    instanceTwo = Deployment.Instance;
                    return Task.FromResult(1);
                });

            tcs.SetResult(1);
            await runTaskOne;

            Assert.NotNull(instanceOne);
            Assert.NotNull(instanceTwo);
            Assert.False(ReferenceEquals(instanceOne, instanceTwo));
        }

        [Fact]
        public async Task DeploymentInstanceIsProtectedFromParallelSynchronousRunAsync()
        {
            var tcs = new TaskCompletionSource<int>();
            var runTaskOne = Deployment.CreateRunnerAndRunAsync(
                () => new Deployment(new MockEngine(), new MockMonitor(new MyMocks()), null),
                runner => tcs.Task);

            // this will throw if we didn't protect
            // the AsyncLocal scope of Deployment.Instance
            await Deployment.CreateRunnerAndRunAsync(
                () => new Deployment(new MockEngine(), new MockMonitor(new MyMocks()), null),
                runner => Task.FromResult(1));

            tcs.SetResult(1);
            await runTaskOne;
        }
    }
}
