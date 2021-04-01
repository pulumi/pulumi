// Copyright 2016-2021, Pulumi Corporation

using Semver;
using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Threading;
using System.Threading.Tasks;
using Pulumi.Automation.Commands.Exceptions;
using Pulumi.Automation.Events;
using Xunit;

namespace Pulumi.Automation.Tests
{
    public class LocalWorkspaceTests
    {
        private static readonly string _dataDirectory =
            Path.Combine(new FileInfo(Assembly.GetExecutingAssembly().Location).DirectoryName, "Data");

        private static readonly string _pulumiOrg = GetTestOrg();

        private static string GetTestSuffix()
        {
            var random = new Random();
            var result = 100000 + random.Next(0, 900000);
            return result.ToString();
        }

        private static string RandomStackName()
        {
            const string chars = "abcdefghijklmnopqrstuvwxyz";
            return new string(Enumerable.Range(1, 8).Select(_ => chars[new Random().Next(chars.Length)]).ToArray());
        }

        private static string GetTestOrg() =>
            Environment.GetEnvironmentVariable("PULUMI_TEST_ORG") ?? "pulumi-test";

        private static string FullyQualifiedStackName(string org, string project, string stack) =>
            $"{org}/{project}/{stack}";

        private static string NormalizeConfigKey(string key, string projectName)
        {
            var parts = key.Split(":");
            if (parts.Length < 2)
                return $"{projectName}:{key}";

            return string.Empty;
        }

        [Theory]
        [InlineData("yaml")]
        [InlineData("yml")]
        [InlineData("json")]
        public async Task GetProjectSettings(string extension)
        {
            var workingDir = Path.Combine(_dataDirectory, extension);
            using var workspace = await LocalWorkspace.CreateAsync(new LocalWorkspaceOptions
            {
                WorkDir = workingDir,
            });

            var settings = await workspace.GetProjectSettingsAsync();
            Assert.NotNull(settings);
            Assert.Equal("testproj", settings!.Name);
            Assert.Equal(ProjectRuntimeName.Go, settings.Runtime.Name);
            Assert.Equal("A minimal Go Pulumi program", settings.Description);
        }

        [Theory]
        [InlineData("yaml")]
        [InlineData("yml")]
        [InlineData("json")]
        public async Task GetStackSettings(string extension)
        {
            var workingDir = Path.Combine(_dataDirectory, extension);
            using var workspace = await LocalWorkspace.CreateAsync(new LocalWorkspaceOptions
            {
                WorkDir = workingDir,
            });

            var settings = await workspace.GetStackSettingsAsync("dev");
            Assert.NotNull(settings);
            Assert.Equal("abc", settings!.SecretsProvider);
            Assert.NotNull(settings.Config);

            Assert.True(settings.Config!.TryGetValue("plain", out var plainValue));
            Assert.Equal("plain", plainValue!.Value);
            Assert.False(plainValue.IsSecure);

            Assert.True(settings.Config.TryGetValue("secure", out var secureValue));
            Assert.Equal("secret", secureValue!.Value);
            Assert.True(secureValue.IsSecure);
        }

        [Fact]
        public async Task AddRemoveListPlugins()
        {
            using var workspace = await LocalWorkspace.CreateAsync();

            var plugins = await workspace.ListPluginsAsync();
            if (plugins.Any(p => p.Name == "aws" && p.Version == "3.0.0"))
            {
                await workspace.RemovePluginAsync("aws", "3.0.0");
                plugins = await workspace.ListPluginsAsync();
                Assert.DoesNotContain(plugins, p => p.Name == "aws" && p.Version == "3.0.0");
            }

            await workspace.InstallPluginAsync("aws", "v3.0.0");
            plugins = await workspace.ListPluginsAsync();
            var aws = plugins.FirstOrDefault(p => p.Name == "aws" && p.Version == "3.0.0");
            Assert.NotNull(aws);

            await workspace.RemovePluginAsync("aws", "3.0.0");
            plugins = await workspace.ListPluginsAsync();
            Assert.DoesNotContain(plugins, p => p.Name == "aws" && p.Version == "3.0.0");
        }

        [Fact]
        public async Task CreateSelectRemoveStack()
        {
            var projectSettings = new ProjectSettings("create_select_remove_stack_test", ProjectRuntimeName.NodeJS);
            using var workspace = await LocalWorkspace.CreateAsync(new LocalWorkspaceOptions
            {
                ProjectSettings = projectSettings,
                EnvironmentVariables = new Dictionary<string, string?>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var stackName = $"{RandomStackName()}";

            var stacks = await workspace.ListStacksAsync();
            if (stacks.Any(s => s.Name == stackName))
            {
                await workspace.RemoveStackAsync(stackName);
                stacks = await workspace.ListStacksAsync();
                Assert.DoesNotContain(stacks, s => s.Name == stackName);
            }

            await workspace.CreateStackAsync(stackName);
            stacks = await workspace.ListStacksAsync();
            var newStack = stacks.FirstOrDefault(s => s.Name == stackName);
            Assert.NotNull(newStack);
            Assert.True(newStack.IsCurrent);

            await workspace.SelectStackAsync(stackName);
            await workspace.RemoveStackAsync(stackName);
            stacks = await workspace.ListStacksAsync();
            Assert.DoesNotContain(stacks, s => s.Name == stackName);
        }

        [Fact]
        public async Task ManipulateConfig()
        {
            var projectName = "manipulate_config_test";
            var projectSettings = new ProjectSettings(projectName, ProjectRuntimeName.NodeJS);

            using var workspace = await LocalWorkspace.CreateAsync(new LocalWorkspaceOptions
            {
                ProjectSettings = projectSettings,
                EnvironmentVariables = new Dictionary<string, string?>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var stackName = $"{RandomStackName()}";
            var stack = await WorkspaceStack.CreateAsync(stackName, workspace);

            var config = new Dictionary<string, ConfigValue>()
            {
                ["plain"] = new ConfigValue("abc"),
                ["secret"] = new ConfigValue("def", isSecret: true),
            };

            var plainKey = NormalizeConfigKey("plain", projectName);
            var secretKey = NormalizeConfigKey("secret", projectName);

            try
            {
                await Assert.ThrowsAsync<CommandException>(
                    () => stack.GetConfigValueAsync(plainKey));

                var values = await stack.GetConfigAsync();
                Assert.Empty(values);

                await stack.SetConfigAsync(config);
                values = await stack.GetConfigAsync();
                Assert.True(values.TryGetValue(plainKey, out var plainValue));
                Assert.Equal("abc", plainValue!.Value);
                Assert.False(plainValue.IsSecret);
                Assert.True(values.TryGetValue(secretKey, out var secretValue));
                Assert.Equal("def", secretValue!.Value);
                Assert.True(secretValue.IsSecret);

                await stack.RemoveConfigValueAsync("plain");
                values = await stack.GetConfigAsync();
                Assert.Single(values);

                await stack.SetConfigValueAsync("foo", new ConfigValue("bar"));
                values = await stack.GetConfigAsync();
                Assert.Equal(2, values.Count);
            }
            finally
            {
                await workspace.RemoveStackAsync(stackName);
            }
        }

        [Fact]
        public async Task ListStackAndCurrentlySelected()
        {
            var projectSettings = new ProjectSettings(
                $"node_list_test{GetTestSuffix()}",
                ProjectRuntimeName.NodeJS);

            using var workspace = await LocalWorkspace.CreateAsync(new LocalWorkspaceOptions
            {
                ProjectSettings = projectSettings,
                EnvironmentVariables = new Dictionary<string, string?>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var stackNames = new List<string>();
            try
            {
                for (var i = 0; i < 2; i++)
                {
                    var stackName = GetStackName();
                    await WorkspaceStack.CreateAsync(stackName, workspace);
                    stackNames.Add(stackName);
                    var summary = await workspace.GetStackAsync();
                    Assert.NotNull(summary);
                    Assert.True(summary!.IsCurrent);
                    var stacks = await workspace.ListStacksAsync();
                    Assert.Equal(i + 1, stacks.Count);
                }
            }
            finally
            {
                foreach (var name in stackNames)
                    await workspace.RemoveStackAsync(name);
            }

            static string GetStackName()
                => $"int_test{GetTestSuffix()}";
        }

        [Fact]
        public async Task CheckStackStatus()
        {
            var projectSettings = new ProjectSettings("check_stack_status_test", ProjectRuntimeName.NodeJS);
            using var workspace = await LocalWorkspace.CreateAsync(new LocalWorkspaceOptions
            {
                ProjectSettings = projectSettings,
                EnvironmentVariables = new Dictionary<string, string?>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var stackName = $"{RandomStackName()}";
            var stack = await WorkspaceStack.CreateAsync(stackName, workspace);
            try
            {
                var history = await stack.GetHistoryAsync();
                Assert.Empty(history);
                var info = await stack.GetInfoAsync();
                Assert.Null(info);
            }
            finally
            {
                await workspace.RemoveStackAsync(stackName);
            }
        }

        [Fact]
        public async Task StackLifecycleLocalProgram()
        {
            var stackName = $"{RandomStackName()}";
            var workingDir = Path.Combine(_dataDirectory, "testproj");
            using var stack = await LocalWorkspace.CreateStackAsync(new LocalProgramArgs(stackName, workingDir)
            {
                EnvironmentVariables = new Dictionary<string, string?>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var config = new Dictionary<string, ConfigValue>()
            {
                ["bar"] = new ConfigValue("abc"),
                ["buzz"] = new ConfigValue("secret", isSecret: true),
            };
            try
            {
                await stack.SetConfigAsync(config);

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
                await stack.Workspace.RemoveStackAsync(stackName);
            }
        }

        [Fact]
        public async Task StackLifecycleInlineProgram()
        {
            var program = PulumiFn.Create(() =>
            {
                var config = new Pulumi.Config();
                return new Dictionary<string, object?>
                {
                    ["exp_static"] = "foo",
                    ["exp_cfg"] = config.Get("bar"),
                    ["exp_secret"] = config.GetSecret("buzz"),
                };
            });

            var stackName = $"{RandomStackName()}";
            var projectName = "inline_node";
            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, stackName, program)
            {
                EnvironmentVariables = new Dictionary<string, string?>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var config = new Dictionary<string, ConfigValue>()
            {
                ["bar"] = new ConfigValue("abc"),
                ["buzz"] = new ConfigValue("secret", isSecret: true),
            };
            try
            {
                await stack.SetConfigAsync(config);

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
                await stack.Workspace.RemoveStackAsync(stackName);
            }
        }

        [Fact(Skip = "Breaking builds")]
        public async Task StackReferenceDestroyDiscardsWithTwoInlinePrograms()
        {
            var programA = PulumiFn.Create(() =>
            {
                return new Dictionary<string, object?>
                {
                    ["exp_static"] = "foo",
                };
            });

            var programB = PulumiFn.Create(() =>
            {
                var config = new Config();
                var stackRef = new StackReference(config.Require("Ref"));
                return new Dictionary<string, object?>
                {
                    ["exp_static"] = stackRef.GetOutput("exp_static"),
                };
            });

            var stackNameA = $"{RandomStackName()}";
            var stackNameB = $"{RandomStackName()}";
            var projectName = "inline_stack_reference";

            var stackA = await SetupStack(projectName, stackNameA, programA, new Dictionary<string, ConfigValue>());

            var stackB = await SetupStack(projectName, stackNameB, programB, new Dictionary<string, ConfigValue>()
            {
                ["Ref"] = new ConfigValue(FullyQualifiedStackName(_pulumiOrg, projectName, stackNameA)),
            });

            try
            {
                // Update the first stack
                {
                    var upResult = await stackA.UpAsync();
                    Assert.Equal(UpdateKind.Update, upResult.Summary.Kind);
                    Assert.Equal(UpdateState.Succeeded, upResult.Summary.Result);
                    Assert.Equal(1, upResult.Outputs.Count);

                    // exp_static
                    Assert.True(upResult.Outputs.TryGetValue("exp_static", out var expStaticValue));
                    Assert.Equal("foo", expStaticValue!.Value);
                    Assert.False(expStaticValue.IsSecret);
                }

                // Update the second stack which references the first
                {
                    var upResult = await stackB.UpAsync();
                    Assert.Equal(UpdateKind.Update, upResult.Summary.Kind);
                    Assert.Equal(UpdateState.Succeeded, upResult.Summary.Result);
                    Assert.Equal(1, upResult.Outputs.Count);

                    // exp_static
                    Assert.True(upResult.Outputs.TryGetValue("exp_static", out var expStaticValue));
                    Assert.Equal("foo", expStaticValue!.Value);
                    Assert.False(expStaticValue.IsSecret);
                }

                // Destroy stacks in reverse order
                {
                    var destroyResult = await stackB.DestroyAsync();
                    Assert.Equal(UpdateKind.Destroy, destroyResult.Summary.Kind);
                    Assert.Equal(UpdateState.Succeeded, destroyResult.Summary.Result);
                }

                {
                    var destroyResult = await stackA.DestroyAsync();
                    Assert.Equal(UpdateKind.Destroy, destroyResult.Summary.Kind);
                    Assert.Equal(UpdateState.Succeeded, destroyResult.Summary.Result);
                }
            }
            // Ensure stacks are deleted even if some of the operations fail
            finally
            {
                await stackA.Workspace.RemoveStackAsync(stackNameA);
                await stackB.Workspace.RemoveStackAsync(stackNameB);
            }

            static async Task<WorkspaceStack> SetupStack(string project, string stackName, PulumiFn program, Dictionary<string, ConfigValue> configMap)
            {
                var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(project, stackName, program)
                {
                    EnvironmentVariables = new Dictionary<string, string>()
                    {
                        ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                    }
                });

                await stack.SetConfigAsync(configMap);

                return stack;
            }
        }

        [Fact]
        public async Task OutputStreamAndDelegateIsWritten()
        {
            var program = PulumiFn.Create(() =>
            {
                return new Dictionary<string, object?>
                {
                    ["test"] = "test",
                };
            });

            var stackName = $"{RandomStackName()}";
            var projectName = "inline_output";
            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, stackName, program)
            {
                EnvironmentVariables = new Dictionary<string, string?>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            try
            {
                var outputCalled = false;

                // pulumi preview
                outputCalled = false;
                var previewResult = await stack.PreviewAsync(new PreviewOptions { OnStandardOutput = (str) => outputCalled = true });
                Assert.False(string.IsNullOrEmpty(previewResult.StandardOutput));
                Assert.True(outputCalled);

                // pulumi up
                outputCalled = false;
                var upResult = await stack.UpAsync(new UpOptions { OnStandardOutput = (str) => outputCalled = true });
                Assert.False(string.IsNullOrEmpty(upResult.StandardOutput));
                Assert.True(outputCalled);

                // pulumi refresh
                outputCalled = false;
                var refreshResult = await stack.RefreshAsync(new RefreshOptions { OnStandardOutput = (str) => outputCalled = true });
                Assert.False(string.IsNullOrEmpty(refreshResult.StandardOutput));
                Assert.True(outputCalled);

                // pulumi destroy
                outputCalled = false;
                var destroyResult = await stack.DestroyAsync(new DestroyOptions { OnStandardOutput = (str) => outputCalled = true });
                Assert.False(string.IsNullOrEmpty(destroyResult.StandardOutput));
                Assert.True(outputCalled);
            }
            finally
            {
                await stack.Workspace.RemoveStackAsync(stack.Name);
            }
        }

        [Fact]
        public async Task HandlesEvents()
        {
            var program = PulumiFn.Create(() =>
            {
                return new Dictionary<string, object?>
                {
                    ["exp_static"] = "foo",
                };
            });
            var projectName = "event_test";
            var stackName = $"inline_events{GetTestSuffix()}";
            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, stackName, program)
            {
                EnvironmentVariables = new Dictionary<string, string>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            try
            {
                // pulumi preview
                var previewResult = await RunCommand<PreviewResult, PreviewOptions>(stack.PreviewAsync, "preview");
                Assert.True(previewResult.ChangeSummary.TryGetValue(OperationType.Create, out var createCount));
                Assert.Equal(1, createCount);

                // pulumi up
                var upResult = await RunCommand<UpResult, UpOptions>(stack.UpAsync, "up");
                Assert.Equal(UpdateKind.Update, upResult.Summary.Kind);
                Assert.Equal(UpdateState.Succeeded, upResult.Summary.Result);

                // pulumi preview
                var previewResultAgain = await RunCommand<PreviewResult, PreviewOptions>(stack.PreviewAsync, "preview");
                Assert.True(previewResultAgain.ChangeSummary.TryGetValue(OperationType.Same, out var sameCount));
                Assert.Equal(1, sameCount);

                // pulumi refresh
                var refreshResult = await RunCommand<UpdateResult, RefreshOptions>(stack.RefreshAsync, "refresh");
                Assert.Equal(UpdateKind.Refresh, refreshResult.Summary.Kind);
                Assert.Equal(UpdateState.Succeeded, refreshResult.Summary.Result);

                // pulumi destroy
                var destroyResult = await RunCommand<UpdateResult, DestroyOptions>(stack.DestroyAsync, "destroy");
                Assert.Equal(UpdateKind.Destroy, destroyResult.Summary.Kind);
                Assert.Equal(UpdateState.Succeeded, destroyResult.Summary.Result);
            }
            finally
            {
                await stack.Workspace.RemoveStackAsync(stackName);
            }

            static async Task<T> RunCommand<T, TOptions>(Func<TOptions, CancellationToken, Task<T>> func, string command)
                where TOptions: UpdateOptions, new()
            {
                var events = new List<EngineEvent>();

                var result = await func(new TOptions() { OnEvent = events.Add }, CancellationToken.None);

                var seenSummaryEvent = events.Any(@event => @event.SummaryEvent != null);
                var seenCancelEvent = events.Any(@event => @event.CancelEvent != null);

                Assert.True(events.Any(), $"No Events found for '{command}'");
                Assert.True(events.SequenceEqual(events.OrderBy(@event => @event.Sequence)), $"Events should be received in the sequence order for '{command}'");
                Assert.True(seenSummaryEvent, $"No SummaryEvent for '{command}'");
                Assert.True(seenCancelEvent, $"No CancelEvent for '{command}'");

                return result;
            }
        }

        private class ValidStack : Stack
        {
            [Output("exp_static")]
            public Output<string> ExpStatic { get; set; }

            [Output("exp_cfg")]
            public Output<string> ExpConfig { get; set; }

            [Output("exp_secret")]
            public Output<string> ExpSecret { get; set; }

            public ValidStack()
            {
                var config = new Pulumi.Config();
                this.ExpStatic = Output.Create("foo");
                this.ExpConfig = Output.Create(config.Get("bar")!);
                this.ExpSecret = config.GetSecret("buzz")!;
            }
        }

        [Fact]
        public async Task StackLifecycleInlineProgramWithTStack()
        {
            var program = PulumiFn.Create<ValidStack>();

            var stackName = $"{RandomStackName()}";
            var projectName = "inline_tstack_node";
            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, stackName, program)
            {
                EnvironmentVariables = new Dictionary<string, string?>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var config = new Dictionary<string, ConfigValue>()
            {
                ["bar"] = new ConfigValue("abc"),
                ["buzz"] = new ConfigValue("secret", isSecret: true),
            };
            try
            {
                await stack.SetConfigAsync(config);

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
                await stack.Workspace.RemoveStackAsync(stackName);
            }
        }

        [Fact]
        public async Task InlineProgramExceptionPropagatesToCaller()
        {
            const string projectName = "exception_inline_node";
            var stackName = $"{RandomStackName()}";
            var program = PulumiFn.Create((Action)(() => throw new FileNotFoundException()));

            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, stackName, program)
            {
                EnvironmentVariables = new Dictionary<string, string?>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var upTask = stack.UpAsync();
            await Assert.ThrowsAsync<FileNotFoundException>(
                () => upTask);
        }

        private class FileNotFoundStack : Pulumi.Stack
        {
            public FileNotFoundStack()
            {
                throw new FileNotFoundException();
            }
        }

        [Fact]
        public async Task InlineProgramExceptionPropagatesToCallerWithTStack()
        {
            const string projectName = "exception_inline_tstack_node";
            var stackName = $"{RandomStackName()}";
            var program = PulumiFn.Create<FileNotFoundStack>();

            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, stackName, program)
            {
                EnvironmentVariables = new Dictionary<string, string?>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var upTask = stack.UpAsync();
            await Assert.ThrowsAsync<FileNotFoundException>(
                () => upTask);
        }

        [Fact(Skip = "Parallel execution is not supported in this first version.")]
        public async Task InlineProgramAllowsParallelExecution()
        {
            const string projectNameOne = "parallel_inline_node1";
            const string projectNameTwo = "parallel_inline_node2";
            var stackNameOne = $"{RandomStackName()}";
            var stackNameTwo = $"{RandomStackName()}";

            var hasReachedSemaphoreOne = false;
            using var semaphoreOne = new SemaphoreSlim(0, 1);

            var programOne = PulumiFn.Create(() =>
            {
                // we want to assert before and after each interaction with
                // the semaphore because we want to alternately stutter
                // programOne and programTwo so we can assert they aren't
                // touching eachothers instances
                var config = new Pulumi.Config();
                Assert.Equal(projectNameOne, Deployment.Instance.ProjectName);
                Assert.Equal(stackNameOne, Deployment.Instance.StackName);
                hasReachedSemaphoreOne = true;
                semaphoreOne.Wait();
                Assert.Equal(projectNameOne, Deployment.Instance.ProjectName);
                Assert.Equal(stackNameOne, Deployment.Instance.StackName);
                return new Dictionary<string, object?>
                {
                    ["exp_static"] = "1",
                    ["exp_cfg"] = config.Get("bar"),
                    ["exp_secret"] = config.GetSecret("buzz"),
                };
            });

            var hasReachedSemaphoreTwo = false;
            using var semaphoreTwo = new SemaphoreSlim(0, 1);

            var programTwo = PulumiFn.Create(() =>
            {
                var config = new Pulumi.Config();
                Assert.Equal(projectNameTwo, Deployment.Instance.ProjectName);
                Assert.Equal(stackNameTwo, Deployment.Instance.StackName);
                hasReachedSemaphoreTwo = true;
                semaphoreTwo.Wait();
                Assert.Equal(projectNameTwo, Deployment.Instance.ProjectName);
                Assert.Equal(stackNameTwo, Deployment.Instance.StackName);
                return new Dictionary<string, object?>
                {
                    ["exp_static"] = "2",
                    ["exp_cfg"] = config.Get("bar"),
                    ["exp_secret"] = config.GetSecret("buzz"),
                };
            });

            using var stackOne = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectNameOne, stackNameOne, programOne)
            {
                EnvironmentVariables = new Dictionary<string, string?>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            using var stackTwo = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectNameTwo, stackNameTwo, programTwo)
            {
                EnvironmentVariables = new Dictionary<string, string?>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            await stackOne.SetConfigAsync(new Dictionary<string, ConfigValue>()
            {
                ["bar"] = new ConfigValue("1"),
                ["buzz"] = new ConfigValue("1", isSecret: true),
            });

            await stackTwo.SetConfigAsync(new Dictionary<string, ConfigValue>()
            {
                ["bar"] = new ConfigValue("2"),
                ["buzz"] = new ConfigValue("2", isSecret: true),
            });

            var upTaskOne = stackOne.UpAsync();
            // wait until we hit semaphore one
            while (!hasReachedSemaphoreOne)
            {
                await Task.Delay(TimeSpan.FromSeconds(2));
                if (upTaskOne.IsFaulted)
                    throw upTaskOne.Exception!;
                else if (upTaskOne.IsCompleted)
                    throw new Exception("Never hit semaphore in first UP task.");
            }

            var upTaskTwo = stackTwo.UpAsync();
            // wait until we hit semaphore two
            while (!hasReachedSemaphoreTwo)
            {
                await Task.Delay(TimeSpan.FromSeconds(2));
                if (upTaskTwo.IsFaulted)
                    throw upTaskTwo.Exception!;
                else if (upTaskTwo.IsCompleted)
                    throw new Exception("Never hit semaphore in second UP task.");
            }

            // alternately allow them to progress
            semaphoreOne.Release();
            var upResultOne = await upTaskOne;

            semaphoreTwo.Release();
            var upResultTwo = await upTaskTwo;

            AssertUpResult(upResultOne, "1");
            AssertUpResult(upResultTwo, "2");

            static void AssertUpResult(UpResult upResult, string value)
            {
                Assert.Equal(UpdateKind.Update, upResult.Summary.Kind);
                Assert.Equal(UpdateState.Succeeded, upResult.Summary.Result);
                Assert.Equal(3, upResult.Outputs.Count);

                // exp_static
                Assert.True(upResult.Outputs.TryGetValue("exp_static", out var expStaticValue));
                Assert.Equal(value, expStaticValue!.Value);
                Assert.False(expStaticValue.IsSecret);

                // exp_cfg
                Assert.True(upResult.Outputs.TryGetValue("exp_cfg", out var expConfigValue));
                Assert.Equal(value, expConfigValue!.Value);
                Assert.False(expConfigValue.IsSecret);

                // exp_secret
                Assert.True(upResult.Outputs.TryGetValue("exp_secret", out var expSecretValue));
                Assert.Equal(value, expSecretValue!.Value);
                Assert.True(expSecretValue.IsSecret);
            }
        }
    
        [Fact]
        public async Task PulumiVersionTest()
        {
            using var workspace = await LocalWorkspace.CreateAsync();
            Assert.Matches("(\\d+\\.)(\\d+\\.)(\\d+)(-.*)?", workspace.PulumiVersion);
        }

        [Theory]
        [InlineData("100.0.0", true)]
        [InlineData("1.0.0", true)]
        [InlineData("2.22.0", false)]
        [InlineData("2.1.0", true)]
        [InlineData("2.21.2", false)]
        [InlineData("2.21.1", false)]
        [InlineData("2.21.0", true)]
        // Note that prerelease < release so this case should error
        [InlineData("2.21.1-alpha.1234", true)]
        public void ValidVersionTheory(string currentVersion, bool errorExpected)
        {
            var testMinVersion = SemVersion.Parse("2.21.1");
            if (errorExpected)
            {
                Action act = () => LocalWorkspace.ValidatePulumiVersion(testMinVersion, currentVersion);
                Assert.Throws<InvalidOperationException>(act);
            }
            else
            {
                LocalWorkspace.ValidatePulumiVersion(testMinVersion, currentVersion);
            }
        }
    }
}
