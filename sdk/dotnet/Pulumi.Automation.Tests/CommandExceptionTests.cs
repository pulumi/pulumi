// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Pulumi.Automation.Commands.Exceptions;
using Xunit;

namespace Pulumi.Automation.Tests
{
    public class CommandExceptionTests
    {
        private static string GetTestSuffix()
        {
            var random = new Random();
            var result = 100000 + random.Next(0, 900000);
            return result.ToString();
        }

        [Fact]
        public async Task StackNotFoundExceptionIsThrown()
        {
            var projectSettings = new ProjectSettings("command_exception_test", ProjectRuntimeName.NodeJS);
            using var workspace = await LocalWorkspace.CreateAsync(new LocalWorkspaceOptions
            {
                ProjectSettings = projectSettings,
            });

            var stackName = $"non_existent_stack{GetTestSuffix()}";
            var selectTask = workspace.SelectStackAsync(stackName);

            await Assert.ThrowsAsync<StackNotFoundException>(
                () => selectTask);
        }

        [Fact]
        public async Task StackAlreadyExistsExceptionIsThrown()
        {
            var projectSettings = new ProjectSettings("command_exception_test", ProjectRuntimeName.NodeJS);
            using var workspace = await LocalWorkspace.CreateAsync(new LocalWorkspaceOptions
            {
                ProjectSettings = projectSettings,
            });

            var stackName = $"already_existing_stack{GetTestSuffix()}";
            await workspace.CreateStackAsync(stackName);

            try
            {
                var createTask = workspace.CreateStackAsync(stackName);
                await Assert.ThrowsAsync<StackAlreadyExistsException>(
                    () => createTask);
            }
            finally
            {
                await workspace.RemoveStackAsync(stackName);
            }
        }

        [Fact]
        public async Task ConcurrentUpdateExceptionIsThrown()
        {
            
            var projectSettings = new ProjectSettings("command_exception_test", ProjectRuntimeName.NodeJS);
            using var workspace = await LocalWorkspace.CreateAsync(new LocalWorkspaceOptions
            {
                ProjectSettings = projectSettings,
            });

            var stackName = $"concurrent_update_stack{GetTestSuffix()}";
            await workspace.CreateStackAsync(stackName);

            try
            {
                var stack = await WorkspaceStack.SelectAsync(stackName, workspace);

                var hitSemaphore = false;
                using var semaphore = new SemaphoreSlim(0, 1);
                var program = PulumiFn.Create(() =>
                {
                    hitSemaphore = true;
                    // ReSharper disable once AccessToDisposedClosure
                    semaphore.Wait();
                    return new Dictionary<string, object?>
                    {
                        ["test"] = "doesnt matter",
                    };
                });

                var upTask = stack.UpAsync(new UpOptions
                {
                    Program = program,
                });

                // wait until we hit semaphore
                while (!hitSemaphore)
                {
                    await Task.Delay(TimeSpan.FromSeconds(2));
                    if (upTask.IsFaulted)
                        throw upTask.Exception!;
                    else if (upTask.IsCompleted)
                        throw new Exception("never hit semaphore in first UP task");
                }

                // before releasing the semaphore, ensure another up throws
                var concurrentTask = stack.UpAsync(new UpOptions
                {
                    Program = program, // should never make it into this
                });

                await Assert.ThrowsAsync<ConcurrentUpdateException>(
                    () => concurrentTask);

                // finish first up call
                semaphore.Release();
                await upTask;
            }
            finally
            {
                await workspace.RemoveStackAsync(stackName);
            }
        }
    }
}
