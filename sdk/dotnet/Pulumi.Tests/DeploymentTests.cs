// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Threading.Tasks;
using Pulumi.Testing;
using Pulumi.Tests.Mocks;
using Xunit;

namespace Pulumi.Tests
{
    public class DeploymentTests
    {
        [Fact]
        public async Task DeploymentInstancePropertyIsProtected()
        {
            // confirm we cannot retrieve deployment instance early
            Assert.Throws<InvalidOperationException>(
                () => _ = Deployment.Instance);

            // confirm we cannot set deployment instance from downstream execution
            var deployment = new Deployment(new MockEngine(), new MockMonitor(new MyMocks()), null);

            var task = Deployment.CreateRunnerAndRunAsync(
                () => deployment,
                _ =>
                {
                    Deployment.Instance = new DeploymentInstance(deployment);
                    return Task.FromResult(1);
                });

            // should not throw until awaited
            await Assert.ThrowsAsync<InvalidOperationException>(
                () => task);
        }

        [Fact]
        public async Task DeploymentInstancesAreSeparate()
        {
            // this test is more of a sanity check that two separate
            // executions have their own deployment instance
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
            // this test is ensuring that CreateRunnerAndRunAsync method is marked async
            var tcs = new TaskCompletionSource<int>();
            var runTaskOne = Deployment.CreateRunnerAndRunAsync(
                () => new Deployment(new MockEngine(), new MockMonitor(new MyMocks()), null),
                runner => tcs.Task);

            // this will throw if we didn't protect
            // the AsyncLocal scope of Deployment.Instance
            // by keeping CreateRunnerAndRunAsync marked async
            await Deployment.CreateRunnerAndRunAsync(
                () => new Deployment(new MockEngine(), new MockMonitor(new MyMocks()), null),
                runner => Task.FromResult(1));

            tcs.SetResult(1);
            await runTaskOne;
        }

        [Fact]
        public async Task RunWaitsForOrphanedOutput()
        {
            var result = 0;
            var tcs = new TaskCompletionSource<int>();
            var runTaskOne = Deployment.CreateRunnerAndRunAsync(
                () => new Deployment(new MockEngine(), new MockMonitor(new MyMocks()), null),
                runner => runner.RunAsync(() =>
                {
                    Output.Create(tcs.Task).Apply(i => result = i);
                    return Task.FromResult((IDictionary<string, object?>)new Dictionary<string, object?>());
                }, null));

            tcs.SetResult(42);
            await runTaskOne;

            Assert.Equal(42, result);
        }

        [Fact]
        public async Task TestAsyncReturnsCreatedResourcesAndOutputs()
        {
            var mocks = new MyMocks();
            var testOptions = new TestOptions();
            var publicIpProperty = "publicIp";
            var (resources, outputs) = await Deployment.TestAsync(mocks, testOptions, () =>
            {
                var instance = new Instance("test-instance", new InstanceArgs());
                return new Dictionary<string, object?>
                {
                    [publicIpProperty] = instance.PublicIp,
                    ["extra"] = 1
                };
            });
            
            // There is only one resource created
            Assert.Single(resources);
            Assert.IsType<Instance>(resources[0]);
            // There are two outputs
            Assert.True(2 == outputs.Count);
            Assert.True(outputs.ContainsKey(publicIpProperty));
            Assert.True(outputs.ContainsKey("extra"));
            if (outputs[publicIpProperty] is Output<string> publicIp)
            {
                var value = await publicIp.GetValueAsync("");
                Assert.Equal(value, MyMocks.InstancePublicIpAddress);
            }
            else
            {
                throw new Exception($"Expected {publicIpProperty} to be of type Output<string>");
            }
            
            Assert.Equal(1, outputs["extra"]);
        }
        
        [Fact]
        public async Task TestAsyncReturnsCreatedResourcesWithoutOuputs()
        {
            var mocks = new MyMocks();
            var testOptions = new TestOptions();
            var resources = await Deployment.TestAsync(mocks, testOptions, () =>
            {
                var instance = new Instance("test-instance", new InstanceArgs());
            });
            
            // There is only one resource created
            Assert.Single(resources);
            Assert.IsType<Instance>(resources[0]);
        }
        
        [Fact]
        public async Task TestAsyncReturnsCreatedResourcesWithoutOuputsInAsyncOverload()
        {
            var mocks = new MyMocks();
            var testOptions = new TestOptions();
            var resources = await Deployment.TestAsync(mocks, testOptions, async() =>
            {
                await Task.Delay(1000);
                var instance = new Instance("test-instance", new InstanceArgs());
            });
            
            // There is only one resource created
            Assert.Single(resources);
            Assert.IsType<Instance>(resources[0]);
        }
        
        [Fact]
        public async Task TestAsyncReturnsCreatedResourcesAndOutputsInAsyncOverload()
        {
            var mocks = new MyMocks();
            var testOptions = new TestOptions();
            var publicIpProperty = "publicIp";
            var (resources, outputs) = await Deployment.TestAsync(mocks, testOptions, async () =>
            {
                await Task.Delay(1000);
                var instance = new Instance("test-instance", new InstanceArgs());
                return new Dictionary<string, object?>
                {
                    [publicIpProperty] = instance.PublicIp,
                    ["extra"] = 1
                };
            });
            
            // There is only one resource created
            Assert.Single(resources);
            Assert.IsType<Instance>(resources[0]);
            // There are two outputs
            Assert.True(2 == outputs.Count);
            Assert.True(outputs.ContainsKey(publicIpProperty));
            Assert.True(outputs.ContainsKey("extra"));
            if (outputs[publicIpProperty] is Output<string> publicIp)
            {
                var value = await publicIp.GetValueAsync("");
                Assert.Equal(value, MyMocks.InstancePublicIpAddress);
            }
            else
            {
                throw new Exception($"Expected {publicIpProperty} to be of type Output<string>");
            }
            
            Assert.Equal(1, outputs["extra"]);
        }
    }
}
