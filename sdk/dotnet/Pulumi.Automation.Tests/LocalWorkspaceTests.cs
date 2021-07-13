// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.IO;
using System.Linq;
using System.Runtime.CompilerServices;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using Pulumi.Automation.Commands.Exceptions;
using Pulumi.Automation.Events;
using Pulumi.Automation.Exceptions;
using Semver;
using Serilog;
using Serilog.Extensions.Logging;
using Xunit;
using Xunit.Abstractions;
using ILogger = Microsoft.Extensions.Logging.ILogger;

namespace Pulumi.Automation.Tests
{
    public class LocalWorkspaceTests
    {
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

        private ILogger TestLogger { get; }

        public LocalWorkspaceTests(ITestOutputHelper output)
        {
            var logger = new LoggerConfiguration()
                .MinimumLevel.Verbose()
                .WriteTo.TestOutput(output, outputTemplate: "[{Timestamp:HH:mm:ss} {Level:u3}] {Message:lj}{NewLine}{Exception}")
                .CreateLogger();

            var loggerFactory = new SerilogLoggerFactory(logger);

            TestLogger = loggerFactory.CreateLogger<LocalWorkspaceTests>();
        }

        [Theory]
        [InlineData("yaml")]
        [InlineData("yml")]
        [InlineData("json")]
        public async Task GetProjectSettings(string extension)
        {
            var workingDir = ResourcePath(Path.Combine("Data", extension));
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
            var workingDir = ResourcePath(Path.Combine("Data", extension));
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
                EnvironmentVariables = new Dictionary<string, string?>
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
            Assert.True(newStack!.IsCurrent);

            await workspace.SelectStackAsync(stackName);
            await workspace.RemoveStackAsync(stackName);
            stacks = await workspace.ListStacksAsync();
            Assert.DoesNotContain(stacks, s => s.Name == stackName);
        }

        [Fact]
        public async Task ImportExportStack()
        {
            var workingDir = ResourcePath(Path.Combine("Data", "testproj"));
            var projectSettings = new ProjectSettings("testproj", ProjectRuntimeName.Go)
            {
                Description = "A minimal Go Pulumi program"
            };
            using var workspace = await LocalWorkspace.CreateAsync(new LocalWorkspaceOptions
            {
                WorkDir = workingDir,
                ProjectSettings = projectSettings
            });

            var stackName = $"{RandomStackName()}";
            try
            {
                var stack = await WorkspaceStack.CreateAsync(stackName, workspace);

                var upResult = await stack.UpAsync();
                Assert.Equal(UpdateKind.Update, upResult.Summary.Kind);
                Assert.Equal(UpdateState.Succeeded, upResult.Summary.Result);
                Assert.Equal(3, upResult.Outputs.Count);

                var deployment = await workspace.ExportStackAsync(stackName);
                Assert.True(deployment.Version > 0);

                var previewBeforeDestroy = await stack.PreviewAsync();
                Assert.Equal(1, previewBeforeDestroy.ChangeSummary[OperationType.Same]);

                await stack.DestroyAsync();

                var previewAfterDestroy = await stack.PreviewAsync();
                Assert.Equal(1, previewAfterDestroy.ChangeSummary[OperationType.Create]);

                await workspace.ImportStackAsync(stackName, deployment);

                // After we imported before-destroy deployment,
                // preview is back to reporting the before-destroy
                // state.

                var previewAfterImport = await stack.PreviewAsync();
                Assert.Equal(1, previewAfterImport.ChangeSummary[OperationType.Same]);
            }
            finally
            {
                await workspace.RemoveStackAsync(stackName);
            }
        }


        [Fact]
        public async Task ManipulateConfig()
        {
            var projectName = "manipulate_config_test";
            var projectSettings = new ProjectSettings(projectName, ProjectRuntimeName.NodeJS);

            using var workspace = await LocalWorkspace.CreateAsync(new LocalWorkspaceOptions
            {
                ProjectSettings = projectSettings,
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var stackName = $"{RandomStackName()}";
            var stack = await WorkspaceStack.CreateAsync(stackName, workspace);

            var plainKey = NormalizeConfigKey("plain", projectName);
            var secretKey = NormalizeConfigKey("secret", projectName);

            try
            {
                await Assert.ThrowsAsync<CommandException>(
                    () => stack.GetConfigAsync(plainKey));

                var values = await stack.GetAllConfigAsync();
                Assert.Empty(values);

                var config = new Dictionary<string, ConfigValue>
                {
                    ["plain"] = new ConfigValue("abc"),
                    ["secret"] = new ConfigValue("def", isSecret: true),
                };
                await stack.SetAllConfigAsync(config);

                values = await stack.GetAllConfigAsync();
                Assert.True(values.TryGetValue(plainKey, out var plainValue));
                Assert.Equal("abc", plainValue!.Value);
                Assert.False(plainValue.IsSecret);
                Assert.True(values.TryGetValue(secretKey, out var secretValue));
                Assert.Equal("def", secretValue!.Value);
                Assert.True(secretValue.IsSecret);

                // Get individual configuration values
                plainValue = await stack.GetConfigAsync(plainKey);
                Assert.Equal("abc", plainValue!.Value);
                Assert.False(plainValue.IsSecret);

                secretValue = await stack.GetConfigAsync(secretKey);
                Assert.Equal("def", secretValue!.Value);
                Assert.True(secretValue.IsSecret);

                await stack.RemoveConfigAsync("plain");
                values = await stack.GetAllConfigAsync();
                Assert.Single(values);

                await stack.SetConfigAsync("foo", new ConfigValue("bar"));
                values = await stack.GetAllConfigAsync();
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
                EnvironmentVariables = new Dictionary<string, string?>
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
                EnvironmentVariables = new Dictionary<string, string?>
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
            var workingDir = ResourcePath(Path.Combine("Data", "testproj"));
            using var stack = await LocalWorkspace.CreateStackAsync(new LocalProgramArgs(stackName, workingDir)
            {
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            try
            {
                var config = new Dictionary<string, ConfigValue>
                {
                    ["bar"] = new ConfigValue("abc"),
                    ["buzz"] = new ConfigValue("secret", isSecret: true),
                };
                await stack.SetAllConfigAsync(config);

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
                var config = new Config();
                return new Dictionary<string, object?>
                {
                    ["exp_static"] = "foo",
                    ["exp_cfg"] = config.Get("bar"),
                    ["exp_secret"] = config.GetSecret("buzz"),
                };
            });
            Assert.IsType<PulumiFnInline>(program);

            var stackName = $"{RandomStackName()}";
            var projectName = "inline_node";
            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, stackName, program)
            {
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            try
            {
                var config = new Dictionary<string, ConfigValue>
                {
                    ["bar"] = new ConfigValue("abc"),
                    ["buzz"] = new ConfigValue("secret", isSecret: true),
                };
                await stack.SetAllConfigAsync(config);

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
        public async Task SupportsStackOutputs()
        {
            var program = PulumiFn.Create(() =>
            {
                var config = new Config();
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
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            try
            {
                var config = new Dictionary<string, ConfigValue>
                {
                    ["bar"] = new ConfigValue("abc"),
                    ["buzz"] = new ConfigValue("secret", isSecret: true),
                };
                await stack.SetAllConfigAsync(config);

                var initialOutputs = await stack.GetOutputsAsync();
                Assert.Empty(initialOutputs);

                // pulumi up
                var upResult = await stack.UpAsync();
                Assert.Equal(UpdateKind.Update, upResult.Summary.Kind);
                Assert.Equal(UpdateState.Succeeded, upResult.Summary.Result);
                AssertOutputs(upResult.Outputs);

                var outputsAfterUp = await stack.GetOutputsAsync();
                AssertOutputs(outputsAfterUp);

                // pulumi destroy
                var destroyResult = await stack.DestroyAsync();
                Assert.Equal(UpdateKind.Destroy, destroyResult.Summary.Kind);
                Assert.Equal(UpdateState.Succeeded, destroyResult.Summary.Result);

                var outputsAfterDestroy = await stack.GetOutputsAsync();
                Assert.Empty(outputsAfterDestroy);
            }
            finally
            {
                await stack.Workspace.RemoveStackAsync(stack.Name);
            }

            static void AssertOutputs(IImmutableDictionary<string, OutputValue> outputs)
            {
                Assert.Equal(3, outputs.Count);

                // exp_static
                Assert.True(outputs.TryGetValue("exp_static", out var expStaticValue));
                Assert.Equal("foo", expStaticValue!.Value);
                Assert.False(expStaticValue.IsSecret);

                // exp_cfg
                Assert.True(outputs.TryGetValue("exp_cfg", out var expConfigValue));
                Assert.Equal("abc", expConfigValue!.Value);
                Assert.False(expConfigValue.IsSecret);

                // exp_secret
                Assert.True(outputs.TryGetValue("exp_secret", out var expSecretValue));
                Assert.Equal("secret", expSecretValue!.Value);
                Assert.True(expSecretValue.IsSecret);
            }
        }

        [Fact(Skip = "Breaking builds")]
        public async Task StackReferenceDestroyDiscardsWithTwoInlinePrograms()
        {
            var programA = PulumiFn.Create(() =>
                new Dictionary<string, object?>
                {
                    ["exp_static"] = "foo",
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

            var stackB = await SetupStack(projectName, stackNameB, programB, new Dictionary<string, ConfigValue>
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
                    EnvironmentVariables = new Dictionary<string, string?>
                    {
                        ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                    }
                });

                await stack.SetAllConfigAsync(configMap);

                return stack;
            }
        }

        [Fact]
        public async Task OutputStreamAndDelegateIsWritten()
        {
            var program = PulumiFn.Create(() =>
                new Dictionary<string, object?>
                {
                    ["test"] = "test",
                });

            var stackName = $"{RandomStackName()}";
            var projectName = "inline_output";
            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, stackName, program)
            {
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            try
            {
                // pulumi preview
                var outputCalled = false;
                var previewResult = await stack.PreviewAsync(new PreviewOptions { OnStandardOutput = str => outputCalled = true });
                Assert.False(string.IsNullOrEmpty(previewResult.StandardOutput));
                Assert.True(outputCalled);

                // pulumi up
                outputCalled = false;
                var upResult = await stack.UpAsync(new UpOptions { OnStandardOutput = str => outputCalled = true });
                Assert.False(string.IsNullOrEmpty(upResult.StandardOutput));
                Assert.True(outputCalled);

                // pulumi refresh
                outputCalled = false;
                var refreshResult = await stack.RefreshAsync(new RefreshOptions { OnStandardOutput = str => outputCalled = true });
                Assert.False(string.IsNullOrEmpty(refreshResult.StandardOutput));
                Assert.True(outputCalled);

                // pulumi destroy
                outputCalled = false;
                var destroyResult = await stack.DestroyAsync(new DestroyOptions { OnStandardOutput = str => outputCalled = true });
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
                new Dictionary<string, object?>
                {
                    ["exp_static"] = "foo",
                });
            var projectName = "event_test";
            var stackName = $"inline_events{GetTestSuffix()}";
            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, stackName, program)
            {
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            try
            {
                // pulumi preview
                var previewResult = await RunCommand(stack.PreviewAsync, "preview", new PreviewOptions());
                Assert.True(previewResult.ChangeSummary.TryGetValue(OperationType.Create, out var createCount));
                Assert.Equal(1, createCount);

                // pulumi up
                var upResult = await RunCommand(stack.UpAsync, "up", new UpOptions());
                Assert.Equal(UpdateKind.Update, upResult.Summary.Kind);
                Assert.Equal(UpdateState.Succeeded, upResult.Summary.Result);

                // pulumi preview
                var previewResultAgain = await RunCommand(stack.PreviewAsync, "preview", new PreviewOptions());
                Assert.True(previewResultAgain.ChangeSummary.TryGetValue(OperationType.Same, out var sameCount));
                Assert.Equal(1, sameCount);

                // pulumi refresh
                var refreshResult = await RunCommand(stack.RefreshAsync, "refresh", new RefreshOptions());
                Assert.Equal(UpdateKind.Refresh, refreshResult.Summary.Kind);
                Assert.Equal(UpdateState.Succeeded, refreshResult.Summary.Result);

                // pulumi destroy
                var destroyResult = await RunCommand(stack.DestroyAsync, "destroy", new DestroyOptions());
                Assert.Equal(UpdateKind.Destroy, destroyResult.Summary.Kind);
                Assert.Equal(UpdateState.Succeeded, destroyResult.Summary.Result);
            }
            finally
            {
                await stack.Workspace.RemoveStackAsync(stackName);
            }

            static async Task<T> RunCommand<T, TOptions>(Func<TOptions, CancellationToken, Task<T>> func, string command, TOptions options)
                where TOptions : UpdateOptions, new()
            {
                var events = new List<EngineEvent>();
                options.OnEvent = events.Add;
                var result = await func(options, CancellationToken.None);

                var seenSummaryEvent = events.Any(@event => @event.SummaryEvent != null);
                var seenCancelEvent = events.Any(@event => @event.CancelEvent != null);

                Assert.True(events.Any(), $"No Events found for '{command}'");
                Assert.True(events.SequenceEqual(events.OrderBy(@event => @event.Sequence)), $"Events should be received in the sequence order for '{command}'");
                Assert.True(seenSummaryEvent, $"No SummaryEvent for '{command}'");
                Assert.True(seenCancelEvent, $"No CancelEvent for '{command}'");

                return result;
            }
        }

        // TODO[pulumi/pulumi#7127]: Re-enable the warning.
        [Fact(Skip="Temporarily skipping test until we've re-enabled the warning - pulumi/pulumi#7127")]
        public async Task ConfigSecretWarnings()
        {
            var program = PulumiFn.Create(() =>
            {
                var config = new Config();

                config.Get("plainstr1");
                config.Require("plainstr2");
                config.GetSecret("plainstr3");
                config.RequireSecret("plainstr4");

                config.GetBoolean("plainbool1");
                config.RequireBoolean("plainbool2");
                config.GetSecretBoolean("plainbool3");
                config.RequireSecretBoolean("plainbool4");

                config.GetInt32("plainint1");
                config.RequireInt32("plainint2");
                config.GetSecretInt32("plainint3");
                config.RequireSecretInt32("plainint4");

                config.GetObject<JsonElement>("plainobj1");
                config.RequireObject<JsonElement>("plainobj2");
                config.GetSecretObject<JsonElement>("plainobj3");
                config.RequireSecretObject<JsonElement>("plainobj4");

                config.Get("str1");
                config.Require("str2");
                config.GetSecret("str3");
                config.RequireSecret("str4");

                config.GetBoolean("bool1");
                config.RequireBoolean("bool2");
                config.GetSecretBoolean("bool3");
                config.RequireSecretBoolean("bool4");

                config.GetInt32("int1");
                config.RequireInt32("int2");
                config.GetSecretInt32("int3");
                config.RequireSecretInt32("int4");

                config.GetObject<JsonElement>("obj1");
                config.RequireObject<JsonElement>("obj2");
                config.GetSecretObject<JsonElement>("obj3");
                config.RequireSecretObject<JsonElement>("obj4");
            });

            var projectName = "inline_dotnet";
            var stackName = $"inline_dotnet{GetTestSuffix()}";
            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, stackName, program)
            {
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            try
            {
                var config = new Dictionary<string, ConfigValue>
                {
                    { "plainstr1", new ConfigValue("1") },
                    { "plainstr2", new ConfigValue("2") },
                    { "plainstr3", new ConfigValue("3") },
                    { "plainstr4", new ConfigValue("4") },
                    { "plainbool1", new ConfigValue("true") },
                    { "plainbool2", new ConfigValue("true") },
                    { "plainbool3", new ConfigValue("true") },
                    { "plainbool4", new ConfigValue("true") },
                    { "plainint1", new ConfigValue("1") },
                    { "plainint2", new ConfigValue("2") },
                    { "plainint3", new ConfigValue("3") },
                    { "plainint4", new ConfigValue("4") },
                    { "plainobj1", new ConfigValue("{}") },
                    { "plainobj2", new ConfigValue("{}") },
                    { "plainobj3", new ConfigValue("{}") },
                    { "plainobj4", new ConfigValue("{}") },
                    { "str1", new ConfigValue("1", isSecret: true) },
                    { "str2", new ConfigValue("2", isSecret: true) },
                    { "str3", new ConfigValue("3", isSecret: true) },
                    { "str4", new ConfigValue("4", isSecret: true) },
                    { "bool1", new ConfigValue("true", isSecret: true) },
                    { "bool2", new ConfigValue("true", isSecret: true) },
                    { "bool3", new ConfigValue("true", isSecret: true) },
                    { "bool4", new ConfigValue("true", isSecret: true) },
                    { "int1", new ConfigValue("1", isSecret: true) },
                    { "int2", new ConfigValue("2", isSecret: true) },
                    { "int3", new ConfigValue("3", isSecret: true) },
                    { "int4", new ConfigValue("4", isSecret: true) },
                    { "obj1", new ConfigValue("{}", isSecret: true) },
                    { "obj2", new ConfigValue("{}", isSecret: true) },
                    { "obj3", new ConfigValue("{}", isSecret: true) },
                    { "obj4", new ConfigValue("{}", isSecret: true) },
                };
                await stack.SetAllConfigAsync(config);

                // pulumi preview
                await RunCommand(stack.PreviewAsync, "preview", new PreviewOptions());

                // pulumi up
                await RunCommand(stack.UpAsync, "up", new UpOptions());
            }
            finally
            {
                await stack.Workspace.RemoveStackAsync(stackName);
            }

            static async Task<T> RunCommand<T, TOptions>(Func<TOptions, CancellationToken, Task<T>> func, string command, TOptions options)
                where TOptions : UpdateOptions, new()
            {
                var expectedWarnings = new[]
                {
                    "Configuration 'inline_dotnet:str1' value is a secret; use `GetSecret` instead of `Get`",
                    "Configuration 'inline_dotnet:str2' value is a secret; use `RequireSecret` instead of `Require`",
                    "Configuration 'inline_dotnet:bool1' value is a secret; use `GetSecretBoolean` instead of `GetBoolean`",
                    "Configuration 'inline_dotnet:bool2' value is a secret; use `RequireSecretBoolean` instead of `RequireBoolean`",
                    "Configuration 'inline_dotnet:int1' value is a secret; use `GetSecretInt32` instead of `GetInt32`",
                    "Configuration 'inline_dotnet:int2' value is a secret; use `RequireSecretInt32` instead of `RequireInt32`",
                    "Configuration 'inline_dotnet:obj1' value is a secret; use `GetSecretObject` instead of `GetObject`",
                    "Configuration 'inline_dotnet:obj2' value is a secret; use `RequireSecretObject` instead of `RequireObject`",
                };

                // These keys should not be in any warning messages.
                var unexpectedWarnings = new[]
                {
                    "plainstr1",
                    "plainstr2",
                    "plainstr3",
                    "plainstr4",
                    "plainbool1",
                    "plainbool2",
                    "plainbool3",
                    "plainbool4",
                    "plainint1",
                    "plainint2",
                    "plainint3",
                    "plainint4",
                    "plainobj1",
                    "plainobj2",
                    "plainobj3",
                    "plainobj4",
                    "str3",
                    "str4",
                    "bool3",
                    "bool4",
                    "int3",
                    "int4",
                    "obj3",
                    "obj4",
                };

                var events = new List<DiagnosticEvent>();
                options.OnEvent = @event =>
                {
                    if (@event.DiagnosticEvent?.Severity == "warning")
                    {
                        events.Add(@event.DiagnosticEvent);
                    }
                };
                var result = await func(options, CancellationToken.None);

                foreach (var expected in expectedWarnings)
                {
                    Assert.Contains(events, @event => @event.Message.Contains(expected));
                }

                foreach (var unexpected in unexpectedWarnings)
                {
                    Assert.DoesNotContain(events, @event => @event.Message.Contains(unexpected));
                }

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
                var config = new Config();
                this.ExpStatic = Output.Create("foo");
                this.ExpConfig = Output.Create(config.Get("bar")!);
                this.ExpSecret = config.GetSecret("buzz")!;
            }
        }

        [Fact]
        public async Task StackLifecycleInlineProgramWithTStack()
        {
            var program = PulumiFn.Create<ValidStack>();
            Assert.IsType<PulumiFn<ValidStack>>(program);

            var stackName = $"{RandomStackName()}";
            var projectName = "inline_tstack_node";
            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, stackName, program)
            {
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            try
            {
                var config = new Dictionary<string, ConfigValue>
                {
                    ["bar"] = new ConfigValue("abc"),
                    ["buzz"] = new ConfigValue("secret", isSecret: true),
                };
                await stack.SetAllConfigAsync(config);

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
        public async Task StackLifecycleInlineProgramWithServiceProvider()
        {
            await using var provider = new ServiceCollection()
                .AddTransient<ValidStack>() // must be transient so it is instantiated each time
                .BuildServiceProvider();

            var program = PulumiFn.Create<ValidStack>(provider);
            Assert.IsType<PulumiFnServiceProvider>(program);

            var stackName = $"{RandomStackName()}";
            var projectName = "inline_serviceprovider_node";
            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, stackName, program)
            {
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            try
            {
                var config = new Dictionary<string, ConfigValue>
                {
                    ["bar"] = new ConfigValue("abc"),
                    ["buzz"] = new ConfigValue("secret", isSecret: true),
                };
                await stack.SetAllConfigAsync(config);

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
            var program = PulumiFn.Create((Action)(() => throw new FileNotFoundException()));
            Assert.IsType<PulumiFnInline>(program);

            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, RandomStackName(), program)
            {
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var previewTask = stack.PreviewAsync();
            await Assert.ThrowsAsync<FileNotFoundException>(
                () => previewTask);

            var upTask = stack.UpAsync();
            await Assert.ThrowsAsync<FileNotFoundException>(
                () => upTask);

            // verify also propagates for output delegates
            var outputTask = Task.Run((Func<string>)(() => throw new FileNotFoundException()));
            var programWithOutput = PulumiFn.Create(() =>
            {
                var output = Output.Create(outputTask);
                return new Dictionary<string, object?>
                {
                    ["output"] = output,
                };
            });
            Assert.IsType<PulumiFnInline>(programWithOutput);

            using var stackWithOutput = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, RandomStackName(), programWithOutput)
            {
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var previewTaskWithOutput = stackWithOutput.PreviewAsync();
            await Assert.ThrowsAsync<FileNotFoundException>(
                () => previewTaskWithOutput);

            var upTaskWithOutput = stackWithOutput.UpAsync();
            await Assert.ThrowsAsync<FileNotFoundException>(
                () => upTaskWithOutput);
        }

        private class FileNotFoundStack : Stack
        {
            public FileNotFoundStack()
            {
                throw new FileNotFoundException();
            }
        }

        private class FileNotFoundOutputStack : Stack
        {
            public FileNotFoundOutputStack()
            {
                var outputTask = Task.Run((Func<string>)(() => throw new FileNotFoundException()));
                var output = Output.Create(outputTask);
                this.RegisterOutputs(new Dictionary<string, object?>
                {
                    ["output"] = output,
                });
            }
        }

        [Fact]
        public async Task InlineProgramExceptionPropagatesToCallerWithTStack()
        {
            const string projectName = "exception_inline_tstack_node";
            var program = PulumiFn.Create<FileNotFoundStack>();
            Assert.IsType<PulumiFn<FileNotFoundStack>>(program);

            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, RandomStackName(), program)
            {
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var previewTask = stack.PreviewAsync();
            await Assert.ThrowsAsync<FileNotFoundException>(
                () => previewTask);

            var upTask = stack.UpAsync();
            await Assert.ThrowsAsync<FileNotFoundException>(
                () => upTask);

            // verify also propagates for output delegates
            var programWithOutput = PulumiFn.Create<FileNotFoundOutputStack>();
            Assert.IsType<PulumiFn<FileNotFoundOutputStack>>(programWithOutput);

            using var stackWithOutput = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, RandomStackName(), programWithOutput)
            {
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var previewTaskWithOutput = stackWithOutput.PreviewAsync();
            await Assert.ThrowsAsync<FileNotFoundException>(
                () => previewTaskWithOutput);

            var upTaskWithOutput = stackWithOutput.UpAsync();
            await Assert.ThrowsAsync<FileNotFoundException>(
                () => upTaskWithOutput);
        }

        [Fact]
        public async Task InlineProgramExceptionPropagatesToCallerWithServiceProvider()
        {
            await using var provider = new ServiceCollection()
                .AddTransient<FileNotFoundStack>() // must be transient so it is instantiated each time
                .AddTransient<FileNotFoundOutputStack>()
                .BuildServiceProvider();

            const string projectName = "exception_inline_serviceprovider_node";
            var program = PulumiFn.Create<FileNotFoundStack>(provider);
            Assert.IsType<PulumiFnServiceProvider>(program);

            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, RandomStackName(), program)
            {
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var previewTask = stack.PreviewAsync();
            await Assert.ThrowsAsync<FileNotFoundException>(
                () => previewTask);

            var upTask = stack.UpAsync();
            await Assert.ThrowsAsync<FileNotFoundException>(
                () => upTask);

            // verify also propagates for output delegates
            var programWithOutput = PulumiFn.Create<FileNotFoundOutputStack>(provider);
            Assert.IsType<PulumiFnServiceProvider>(programWithOutput);

            using var stackWithOutput = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, RandomStackName(), programWithOutput)
            {
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var previewTaskWithOutput = stackWithOutput.PreviewAsync();
            await Assert.ThrowsAsync<FileNotFoundException>(
                () => previewTaskWithOutput);

            var upTaskWithOutput = stackWithOutput.UpAsync();
            await Assert.ThrowsAsync<FileNotFoundException>(
                () => upTaskWithOutput);
        }

        [Fact]
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
                var config = new Config();
                Assert.Equal(projectNameOne, Deployment.Instance.ProjectName);
                Assert.Equal(stackNameOne, Deployment.Instance.StackName);
                hasReachedSemaphoreOne = true;
                // ReSharper disable once AccessToDisposedClosure
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
                var config = new Config();
                Assert.Equal(projectNameTwo, Deployment.Instance.ProjectName);
                Assert.Equal(stackNameTwo, Deployment.Instance.StackName);
                hasReachedSemaphoreTwo = true;
                // ReSharper disable once AccessToDisposedClosure
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
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            using var stackTwo = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectNameTwo, stackNameTwo, programTwo)
            {
                EnvironmentVariables = new Dictionary<string, string?>
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            await stackOne.SetAllConfigAsync(new Dictionary<string, ConfigValue>
            {
                ["bar"] = new ConfigValue("1"),
                ["buzz"] = new ConfigValue("1", isSecret: true),
            });

            await stackTwo.SetAllConfigAsync(new Dictionary<string, ConfigValue>
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
                if (upTaskOne.IsCompleted)
                    throw new Exception("Never hit semaphore in first UP task.");
            }

            var upTaskTwo = stackTwo.UpAsync();
            // wait until we hit semaphore two
            while (!hasReachedSemaphoreTwo)
            {
                await Task.Delay(TimeSpan.FromSeconds(2));
                if (upTaskTwo.IsFaulted)
                    throw upTaskTwo.Exception!;
                if (upTaskTwo.IsCompleted)
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

        [Fact(Skip = "Flakey test - https://github.com/pulumi/pulumi/issues/7467")]
        public async Task WorkspaceStackSupportsCancel()
        {
            var workingDir = ResourcePath(Path.Combine("Data", "testproj"));
            using var workspace = await LocalWorkspace.CreateAsync(new LocalWorkspaceOptions
            {
                WorkDir = workingDir
            });
            var stackName = $"{RandomStackName()}";

            var stack = await WorkspaceStack.CreateAsync(stackName, workspace);
            try
            {
                await stack.UpAsync();

                // Race destroy and cancel. We do not know for sure
                // which one will win.
                var destroyTask = stack.DestroyAsync();

                // Empirically 1s delay makes cancel task win
                // sometimes. YMMV.
                await Task.Delay(1000);

                var cancelTask = stack.CancelAsync();

                try
                {
                    Task.WaitAll(destroyTask, cancelTask);
                }
                catch (AggregateException)
                {
                }

                if (destroyTask.IsFaulted && !cancelTask.IsFaulted)
                {
                    // This is what we want to happen at least
                    // sometimes to test Cancel functionality.
                }
                else if (cancelTask.IsFaulted && !destroyTask.IsFaulted)
                {
                    // This may happen if destroyTask wins the race.
                }
                else
                {
                    // This should not happen.
                    Assert.True(destroyTask.IsFaulted, "destroyTask must fail");
                    Assert.True(!cancelTask.IsFaulted, "cancelTask must not fail");
                }
            }
            finally
            {
                await workspace.RemoveStackAsync(stackName);
            }
        }

        [Fact]
        public async Task PulumiVersionTest()
        {
            using var workspace = await LocalWorkspace.CreateAsync();
            Assert.Matches("(\\d+\\.)(\\d+\\.)(\\d+)(-.*)?", workspace.PulumiVersion);
        }

        [Theory]
        [InlineData("100.0.0", true, false)]
        [InlineData("1.0.0", true, false)]
        [InlineData("2.22.0", false, false)]
        [InlineData("2.1.0", true, false)]
        [InlineData("2.21.2", false, false)]
        [InlineData("2.21.1", false, false)]
        [InlineData("2.21.0", true, false)]
        // Note that prerelease < release so this case should error
        [InlineData("2.21.1-alpha.1234", true, false)]
        [InlineData("2.20.0", false, true)]
        [InlineData("2.22.0", false, true)]
        public void ValidVersionTheory(string currentVersion, bool errorExpected, bool optOut)
        {
            var testMinVersion = SemVersion.Parse("2.21.1");
            if (errorExpected)
            {
                void ValidatePulumiVersion() => LocalWorkspace.ValidatePulumiVersion(testMinVersion, currentVersion, optOut);
                Assert.Throws<InvalidOperationException>(ValidatePulumiVersion);
            }
            else
            {
                LocalWorkspace.ValidatePulumiVersion(testMinVersion, currentVersion, optOut);
            }
        }

        [Fact]
        public async Task RespectsProjectSettingsTest()
        {
            var program = PulumiFn.Create<ValidStack>();

            var stackName = $"{RandomStackName()}";
            var projectName = "project_was_overwritten";

            var workdir = ResourcePath(Path.Combine("Data", "correct_project"));

            var stack = await LocalWorkspace.CreateStackAsync(
                new InlineProgramArgs(projectName, stackName, program)
                {
                    WorkDir = workdir
                });

            var settings = await stack.Workspace.GetProjectSettingsAsync();
            Assert.Equal("correct_project", settings!.Name);
            Assert.Equal("This is a description", settings.Description);
        }

        [Fact]
        public async Task DetectsProjectSettingConflictTest()
        {
            var program = PulumiFn.Create<ValidStack>();

            var stackName = $"{RandomStackName()}";
            var projectName = "project_was_overwritten";

            var workdir = ResourcePath(Path.Combine("Data", "correct_project"));

            var projectSettings = ProjectSettings.Default(projectName);
            projectSettings.Description = "non-standard description";

            await Assert.ThrowsAsync<ProjectSettingsConflictException>(() =>
                LocalWorkspace.CreateStackAsync(
                    new InlineProgramArgs(projectName, stackName, program)
                    {
                        WorkDir = workdir,
                        ProjectSettings = projectSettings
                    })
            );
        }

        [Fact]
        public async Task InlineProgramLoggerCanBeOverridden()
        {
            var program = PulumiFn.Create(() =>
            {
                Log.Debug("test");
            });

            var loggerWasInvoked = false;
            var logger = new CustomLogger(() => loggerWasInvoked = true);

            var stackName = $"{RandomStackName()}";
            var projectName = "inline_logger_override";

            using var stack = await LocalWorkspace.CreateOrSelectStackAsync(
                new InlineProgramArgs(projectName, stackName, program)
                {
                    Logger = logger,
                });

            // make sure workspace logger is used
            await stack.PreviewAsync();
            Assert.True(loggerWasInvoked);

            // preview logger is used
            loggerWasInvoked = false;
            stack.Workspace.Logger = null;
            await stack.PreviewAsync(new PreviewOptions
            {
                Logger = logger,
            });
            Assert.True(loggerWasInvoked);

            // up logger is used
            loggerWasInvoked = false;
            await stack.UpAsync(new UpOptions
            {
                Logger = logger,
            });
            Assert.True(loggerWasInvoked);

            await stack.DestroyAsync();
        }

        [Fact]
        public async Task InlineProgramLoggerCanRedirectToTestOutput()
        {
            var program = PulumiFn.Create(() =>
            {
                Log.Info("Pulumi.Log calls appear in test output");
            });

            var stackName = $"{RandomStackName()}";
            var projectName = "inline_logger_test_output";

            using var stack = await LocalWorkspace.CreateOrSelectStackAsync(
                new InlineProgramArgs(projectName, stackName, program)
                {
                    Logger = TestLogger
                });

            TestLogger.LogInformation("Previewing stack...");
            await stack.PreviewAsync();

            TestLogger.LogInformation("Updating stack...");
            await stack.UpAsync();

            TestLogger.LogInformation("Destroying stack...");
            await stack.DestroyAsync();
        }

        [Fact]
        public async Task InlineProgramExceptionDuringUpShouldNotDeleteResources()
        {
            var program = PulumiFn.Create(() =>
            {
                var config = new Config();
                var a = new ComponentResource("test:res:a", "a", null);

                if (config.GetBoolean("ShouldFail") == true)
                    throw new FileNotFoundException("ShouldFail");

                var b = new ComponentResource("test:res:b", "b", null);
                var c = new ComponentResource("test:res:c", "c", null);
            });
            Assert.IsType<PulumiFnInline>(program);

            var stackName = $"{RandomStackName()}";
            var projectName = "test_optionally_failing_stack";
            using var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, stackName, program)
            {
                EnvironmentVariables = new Dictionary<string, string?>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var config = new Dictionary<string, ConfigValue>()
            {
                ["ShouldFail"] = new ConfigValue("false"),
            };
            try
            {
                await stack.SetAllConfigAsync(config);

                // pulumi up
                var upResult = await stack.UpAsync();
                Assert.Equal(UpdateKind.Update, upResult.Summary.Kind);
                Assert.Equal(UpdateState.Succeeded, upResult.Summary.Result);
                Assert.True(upResult.Summary.ResourceChanges!.TryGetValue(OperationType.Create, out var upCount));
                Assert.Equal(4, upCount);

                config["ShouldFail"] = new ConfigValue("true");

                await stack.SetAllConfigAsync(config);
                var sb = new System.Text.StringBuilder();

                await Assert.ThrowsAsync<FileNotFoundException>(
                    () => stack.UpAsync(new UpOptions()
                    {
                        OnStandardOutput = msg => sb.AppendLine(msg),
                    }));

                var upOutput = sb.ToString();
                Assert.DoesNotContain("test:res:b b deleted", upOutput);
                Assert.DoesNotContain("test:res:c c deleted", upOutput);

                config["ShouldFail"] = new ConfigValue("false");

                await stack.SetAllConfigAsync(config);

                // pulumi preview
                var previewResult = await stack.PreviewAsync(new PreviewOptions() { OnStandardOutput = Console.WriteLine });
                Assert.True(previewResult.ChangeSummary.TryGetValue(OperationType.Same, out var sameCount));
                Assert.Equal(4, sameCount);
            }
            finally
            {
                var destroyResult = await stack.DestroyAsync();
                Assert.Equal(UpdateKind.Destroy, destroyResult.Summary.Kind);
                Assert.Equal(UpdateState.Succeeded, destroyResult.Summary.Result);
                await stack.Workspace.RemoveStackAsync(stackName);
            }
        }

        private string ResourcePath(string path, [CallerFilePath] string pathBase = "LocalWorkspaceTests.cs")
        {
            var dir = Path.GetDirectoryName(pathBase) ?? ".";
            return Path.Combine(dir, path);
        }

        private class CustomLogger : ILogger
        {
            private readonly Action _action;

            public CustomLogger(Action action)
            {
                _action = action;
            }

            public IDisposable BeginScope<TState>(TState state)
            {
                throw new NotImplementedException();
            }

            public bool IsEnabled(LogLevel logLevel)
                => true;

            public void Log<TState>(LogLevel logLevel, EventId eventId, TState state, Exception exception, Func<TState, Exception, string> formatter)
                => _action();
        }
    }
}
