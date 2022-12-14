// Copyright 2016-2022, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Threading.Tasks;
using Xunit;

using static Pulumi.Automation.Tests.Utility;

namespace Pulumi.Automation.Tests
{
    public class RemoteWorkspaceTests
    {
        private const string _testRepo = "https://github.com/pulumi/test-repo.git";

        public static IEnumerable<object[]> ErrorsData()
        {
            const string stackName = "owner/project/stack";

            var factories = new Func<RemoteGitProgramArgs, Task<RemoteWorkspaceStack>>[]
            {
                RemoteWorkspace.CreateStackAsync,
                RemoteWorkspace.CreateOrSelectStackAsync,
                RemoteWorkspace.SelectStackAsync,
            };

            var tests = new[]
            {
                new
                {
                    Args = new RemoteGitProgramArgs(null!, _testRepo),
                    Error = "StackName \"\" not fully qualified.",
                },
                new
                {
                    Args = new RemoteGitProgramArgs("", _testRepo),
                    Error = "StackName \"\" not fully qualified.",
                },
                new
                {
                    Args = new RemoteGitProgramArgs("name", _testRepo),
                    Error = "StackName \"name\" not fully qualified.",
                },
                new
                {
                    Args = new RemoteGitProgramArgs("owner/name", _testRepo),
                    Error = "StackName \"owner/name\" not fully qualified.",
                },
                new
                {
                    Args = new RemoteGitProgramArgs("/", _testRepo),
                    Error = "StackName \"/\" not fully qualified.",
                },
                new
                {
                    Args = new RemoteGitProgramArgs("//", _testRepo),
                    Error = "StackName \"//\" not fully qualified.",
                },
                new
                {
                    Args = new RemoteGitProgramArgs("///", _testRepo),
                    Error = "StackName \"///\" not fully qualified.",
                },
                new
                {
                    Args = new RemoteGitProgramArgs("owner/project/stack/wat", _testRepo),
                    Error = "StackName \"owner/project/stack/wat\" not fully qualified.",
                },
                new
                {
                    Args = new RemoteGitProgramArgs(stackName, null!),
                    Error = "Url is required.",
                },
                new
                {
                    Args = new RemoteGitProgramArgs(stackName, ""),
                    Error = "Url is required.",
                },
                new
                {
                    Args = new RemoteGitProgramArgs(stackName, _testRepo),
                    Error = "either Branch or CommitHash is required.",
                },
                new
                {
                    Args = new RemoteGitProgramArgs(stackName, _testRepo)
                    {
                        Branch = "",
                        CommitHash = "",
                    },
                    Error = "either Branch or CommitHash is required.",
                },
                new
                {
                    Args = new RemoteGitProgramArgs(stackName, _testRepo)
                    {
                        Branch = "branch",
                        CommitHash = "commit",
                    },
                    Error = "Branch and CommitHash cannot both be specified.",
                },
                new
                {
                    Args = new RemoteGitProgramArgs(stackName, _testRepo)
                    {
                        Branch = "branch",
                        Auth = new RemoteGitAuthArgs
                        {
                            SshPrivateKey = "key",
                            SshPrivateKeyPath = "path",
                        },
                    },
                    Error = "SshPrivateKey and SshPrivateKeyPath cannot both be specified.",
                },
            };

            foreach (var factory in factories)
            {
                foreach (var test in tests)
                {
                    yield return new object[] { factory, test.Args, test.Error };
                }
            }
        }

        [Theory]
        [MemberData(nameof(ErrorsData))]
        public async Task Errors(Func<RemoteGitProgramArgs, Task<RemoteWorkspaceStack>> factory,
            RemoteGitProgramArgs args, string error)
        {
            var exception = await Assert.ThrowsAsync<ArgumentException>(() => factory(args));
            Assert.Equal(error, exception.Message);
        }

        // This test requires the service with access to Pulumi Deployments.
        // Set PULUMI_ACCESS_TOKEN to an access token with access to Pulumi Deployments
        // and set PULUMI_TEST_DEPLOYMENTS_API to any value to enable the test.
        [DeploymentsApiFact]
        public Task CreateStackLifecycle()
            => TestStackLifeCycle(RemoteWorkspace.CreateStackAsync);

        // This test requires the service with access to Pulumi Deployments.
        // Set PULUMI_ACCESS_TOKEN to an access token with access to Pulumi Deployments
        // and set PULUMI_TEST_DEPLOYMENTS_API to any value to enable the test.
        [DeploymentsApiFact]
        public Task CreateOrSelectStackLifecycle()
            => TestStackLifeCycle(RemoteWorkspace.CreateOrSelectStackAsync);

        private static async Task TestStackLifeCycle(Func<RemoteGitProgramArgs, Task<RemoteWorkspaceStack>> factory)
        {
            var stackName =  FullyQualifiedStackName(GetTestOrg(), "go_remote_proj", RandomStackName());
            using var stack = await factory(new RemoteGitProgramArgs(stackName, _testRepo)
            {
                Branch = "refs/heads/master",
                ProjectPath = "goproj",
                PreRunCommands =
                {
                    $"pulumi config set bar abc --stack {stackName}",
                    $"pulumi config set --secret buzz secret --stack {stackName}",
                },
            });

            try
            {
                // pulumi up
                var upResult = await stack.UpAsync();
                Assert.Equal(UpdateKind.Update, upResult.Summary.Kind);
                Assert.Equal(UpdateState.Succeeded, upResult.Summary.Result);
                Assert.Equal(3, upResult.Outputs.Count);

                // exp_static
                Assert.True(upResult.Outputs.TryGetValue("exp_static", out var expStaticValue));
                Assert.Equal("foo", expStaticValue!.Value);
                Assert.False(expStaticValue.IsSecret);

                // exp_cfg
                Assert.True(upResult.Outputs.TryGetValue("exp_cfg", out var expConfigValue));
                Assert.Equal("abc", expConfigValue!.Value);
                Assert.False(expConfigValue.IsSecret);

                // exp_secret
                Assert.True(upResult.Outputs.TryGetValue("exp_secret", out var expSecretValue));
                Assert.Equal("secret", expSecretValue!.Value);
                Assert.True(expSecretValue.IsSecret);

                // pulumi preview
                var previewResult = await stack.PreviewAsync();
                Assert.True(previewResult.ChangeSummary.TryGetValue(OperationType.Same, out var sameCount));
                Assert.Equal(1, sameCount);

                // pulumi refresh
                var refreshResult = await stack.RefreshAsync();
                Assert.Equal(UpdateKind.Refresh, refreshResult.Summary.Kind);
                Assert.Equal(UpdateState.Succeeded, refreshResult.Summary.Result);

                // pulumi destroy
                var destroyResult = await stack.DestroyAsync();
                Assert.Equal(UpdateKind.Destroy, destroyResult.Summary.Kind);
                Assert.Equal(UpdateState.Succeeded, destroyResult.Summary.Result);
            }
            finally
            {
                using var local = await LocalWorkspace.CreateAsync();
                await local.RemoveStackAsync(stackName);
            }
        }

        [Theory]
        [InlineData("owner/project/stack", true)]
        [InlineData("", false)]
        [InlineData("name", false)]
        [InlineData("owner/name", false)]
        [InlineData("/", false)]
        [InlineData("//", false)]
        [InlineData("///", false)]
        [InlineData("owner/project/stack/wat", false)]
        public void IsFullyQualifiedStackName(string input, bool expected)
        {
            var actual = RemoteWorkspace.IsFullyQualifiedStackName(input);
            Assert.Equal(expected, actual);
        }
    }
}
