// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Text;
using Pulumi.Automation.Serialization;
using Xunit;
using Xunit.Abstractions;

namespace Pulumi.Automation.Tests.Serialization
{
    public class ProjectRuntimeYamlConverterTests
    {
        private readonly ITestOutputHelper _output;
        private static readonly LocalSerializer _serializer = new LocalSerializer();

        public ProjectRuntimeYamlConverterTests(ITestOutputHelper output)
        {
            _output = output;
        }

        [Theory]
        [InlineData(ProjectRuntimeName.NodeJS)]
        [InlineData(ProjectRuntimeName.Go)]
        [InlineData(ProjectRuntimeName.Python)]
        [InlineData(ProjectRuntimeName.Dotnet)]
        public void CanDeserializeWithStringRuntime(ProjectRuntimeName runtimeName)
        {
            var yaml = $@"
name: test-project
runtime: {runtimeName.ToString().ToLower()}
";

            var model = _serializer.DeserializeYaml<ProjectSettingsModel>(yaml);
            var settings = model.Convert();
            Assert.NotNull(settings);
            Assert.IsType<ProjectSettings>(settings);
            Assert.Equal("test-project", settings.Name);
            Assert.Equal(runtimeName, settings.Runtime.Name);
            Assert.Null(settings.Runtime.Options);
        }

        [Theory]
        [InlineData(ProjectRuntimeName.NodeJS)]
        [InlineData(ProjectRuntimeName.Go)]
        [InlineData(ProjectRuntimeName.Python)]
        [InlineData(ProjectRuntimeName.Dotnet)]
        public void CanDeserializeWithObjectRuntime(ProjectRuntimeName runtimeName)
        {
            var yaml = $@"
name: test-project
runtime:
  name: {runtimeName.ToString().ToLower()}
  options:
    typescript: true
    binary: test-binary
    virtualenv: test-env
";

            var model = _serializer.DeserializeYaml<ProjectSettingsModel>(yaml);
            var settings = model.Convert();
            Assert.NotNull(settings);
            Assert.IsType<ProjectSettings>(settings);
            Assert.Equal("test-project", settings.Name);
            Assert.Equal(runtimeName, settings.Runtime.Name);
            Assert.NotNull(settings.Runtime.Options);
            Assert.Equal(true, settings.Runtime.Options!.TypeScript);
            Assert.Equal("test-binary", settings.Runtime.Options.Binary);
            Assert.Equal("test-env", settings.Runtime.Options.VirtualEnv);
        }

        [Fact]
        public void SerializesAsStringIfOptionsNull()
        {
            var runtime = new ProjectRuntime(ProjectRuntimeName.Dotnet);

            var yaml = _serializer.SerializeYaml(runtime);
            _output.WriteLine(yaml);

            Assert.Equal("dotnet" + Environment.NewLine, yaml);
        }

        [Fact]
        public void SerializesAsObjectIfOptionsNotNull()
        {
            var runtime = new ProjectRuntime(ProjectRuntimeName.Dotnet)
            {
                Options = new ProjectRuntimeOptions
                {
                    TypeScript = true,
                },
            };

            var yaml = _serializer.SerializeYaml(runtime);
            _output.WriteLine(yaml);

            var expected = new StringBuilder();
            expected.AppendLine("name: dotnet");
            expected.AppendLine("options:");
            expected.AppendLine("  typescript: true");

            Assert.Equal(expected.ToString(), yaml);
        }
    }
}
