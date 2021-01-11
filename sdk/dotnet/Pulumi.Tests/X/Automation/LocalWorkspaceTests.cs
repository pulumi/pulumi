using System;
using System.Collections.Generic;
using System.IO;
using System.Reflection;
using System.Threading.Tasks;
using Pulumi.X.Automation;
using Pulumi.X.Automation.Commands.Exceptions;
using Xunit;

namespace Pulumi.Tests.X.Automation
{
    public class LocalWorkspaceTests
    {
        // TODO: Change this path when the Automation API is promoted to release (remove X directory)
        private static readonly string DataDirectory =
            Path.Combine(new FileInfo(Assembly.GetExecutingAssembly().Location).DirectoryName, "X", "Automation", "Data");

        private static string GetTestSuffix()
        {
            var random = new Random();
            var result = 100000 + random.Next(0, 900000);
            return result.ToString();
        }

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
            var workingDir = Path.Combine(DataDirectory, extension);
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
            var workingDir = Path.Combine(DataDirectory, extension);
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
            await workspace.InstallPluginAsync("aws", "v3.0.0");
            await workspace.RemovePluginAsync("aws", "3.0.0");
            await workspace.ListPluginsAsync();
        }

        [Fact]
        public async Task CreateSelectRemoveStack()
        {
            var projectSettings = new ProjectSettings("node_test", ProjectRuntimeName.NodeJS);
            using var workspace = await LocalWorkspace.CreateAsync(new LocalWorkspaceOptions
            {
                ProjectSettings = projectSettings,
                EnvironmentVariables = new Dictionary<string, string>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var stackName = $"int_test{GetTestSuffix()}";
            await workspace.CreateStackAsync(stackName);
            await workspace.SelectStackAsync(stackName);
            await workspace.RemoveStackAsync(stackName);
        }

        [Fact]
        public async Task ManipulateConfig()
        {
            var projectName = "node_test";
            var projectSettings = new ProjectSettings(projectName, ProjectRuntimeName.NodeJS);

            using var workspace = await LocalWorkspace.CreateAsync(new LocalWorkspaceOptions
            {
                ProjectSettings = projectSettings,
                EnvironmentVariables = new Dictionary<string, string>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var stackName = $"int_test{GetTestSuffix()}";
            var stack = await XStack.CreateAsync(stackName, workspace);

            var config = new Dictionary<string, ConfigValue>()
            {
                ["plain"] = new ConfigValue("abc"),
                ["secret"] = new ConfigValue("def", isSecret: true),
            };

            var plainKey = NormalizeConfigKey("plain", projectName);
            var secretKey = NormalizeConfigKey("secret", projectName);

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

            await workspace.RemoveStackAsync(stackName);
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
                EnvironmentVariables = new Dictionary<string, string>()
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
                    await XStack.CreateAsync(stackName, workspace);
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
            var projectSettings = new ProjectSettings("node_test", ProjectRuntimeName.NodeJS);
            using var workspace = await LocalWorkspace.CreateAsync(new LocalWorkspaceOptions
            {
                ProjectSettings = projectSettings,
                EnvironmentVariables = new Dictionary<string, string>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var stackName = $"int_test{GetTestSuffix()}";
            var stack = await XStack.CreateAsync(stackName, workspace);
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
            var stackName = $"int_test{GetTestSuffix()}";
            var workingDir = Path.Combine(DataDirectory, "testproj");
            using var stack = await LocalWorkspace.CreateStackAsync(new LocalProgramArgs(stackName, workingDir)
            {
                EnvironmentVariables = new Dictionary<string, string>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var config = new Dictionary<string, ConfigValue>()
            {
                ["bar"] = new ConfigValue("abc"),
                ["buzz"] = new ConfigValue("secret", isSecret: true),
            };
            await stack.SetConfigAsync(config);

            // pulumi up
            var upResult = await stack.UpAsync();
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
            await stack.PreviewAsync();
            // TODO: update assertions when we have structured output

            // pulumi refresh
            var refreshResult = await stack.RefreshAsync();
            Assert.Equal(UpdateKind.Refresh, refreshResult.Summary.Kind);
            Assert.Equal(UpdateState.Succeeded, refreshResult.Summary.Result);

            // pulumi destroy
            var destroyResult = await stack.DestroyAsync();
            Assert.Equal(UpdateKind.Destroy, destroyResult.Summary.Kind);
            Assert.Equal(UpdateState.Succeeded, destroyResult.Summary.Result);

            await stack.Workspace.RemoveStackAsync(stackName);
        }

        [Fact]
        public async Task StackLifecycleInlineProgram()
        {
            PulumiFn program = () =>
            {
                var config = new Pulumi.Config();
                return new Dictionary<string, object?>
                {
                    ["exp_static"] = "foo",
                    ["exp_cfg"] = config.Get("bar"),
                    ["exp_secret"] = config.GetSecret("buzz"),
                };
            };

            var stackName = $"int_test{GetTestSuffix()}";
            var projectName = "inline_node";
            var stack = await LocalWorkspace.CreateStackAsync(new InlineProgramArgs(projectName, stackName, program)
            {
                EnvironmentVariables = new Dictionary<string, string>()
                {
                    ["PULUMI_CONFIG_PASSPHRASE"] = "test",
                }
            });

            var config = new Dictionary<string, ConfigValue>()
            {
                ["bar"] = new ConfigValue("abc"),
                ["buzz"] = new ConfigValue("secret", isSecret: true),
            };
            await stack.SetConfigAsync(config);

            // pulumi up
            var upResult = await stack.UpAsync();
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
            await stack.PreviewAsync();
            // TODO: update assertions when we have structured output

            // pulumi refresh
            var refreshResult = await stack.RefreshAsync();
            Assert.Equal(UpdateKind.Refresh, refreshResult.Summary.Kind);
            Assert.Equal(UpdateState.Succeeded, refreshResult.Summary.Result);

            // pulumi destroy
            var destroyResult = await stack.DestroyAsync();
            Assert.Equal(UpdateKind.Destroy, destroyResult.Summary.Kind);
            Assert.Equal(UpdateState.Succeeded, destroyResult.Summary.Result);

            await stack.Workspace.RemoveStackAsync(stackName);
        }
    }
}
