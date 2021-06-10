// Copyright 2016-2021, Pulumi Corporation

using System.Text.Json;
using Pulumi.Automation.Serialization;
using Xunit;
using Xunit.Abstractions;

namespace Pulumi.Automation.Tests.Serialization
{
    public class ProjectRuntimeJsonConverterTests
    {
        private readonly ITestOutputHelper _output;
        private static readonly LocalSerializer _serializer = new LocalSerializer();

        public ProjectRuntimeJsonConverterTests(ITestOutputHelper output)
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
            var json = $@"
{{
    ""name"": ""test-project"",
    ""runtime"": ""{runtimeName.ToString().ToLower()}""
}}
";

            var settings = _serializer.DeserializeJson<ProjectSettings>(json);
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
            var json = $@"
{{
    ""name"": ""test-project"",
    ""runtime"": {{
        ""name"": ""{runtimeName.ToString().ToLower()}"",
        ""options"": {{
            ""typeScript"": true,
            ""binary"": ""test-binary"",
            ""virtualEnv"": ""test-env""
        }}
    }}
}}
";

            var settings = _serializer.DeserializeJson<ProjectSettings>(json);
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

            var json = _serializer.SerializeJson(runtime);
            _output.WriteLine(json);

            using var document = JsonDocument.Parse(json);
            Assert.NotNull(document);
            Assert.Equal(JsonValueKind.String, document.RootElement.ValueKind);
            Assert.Equal("dotnet", document.RootElement.GetString());
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

            var json = _serializer.SerializeJson(runtime);
            _output.WriteLine(json);

            using var document = JsonDocument.Parse(json);
            Assert.NotNull(document);
            Assert.Equal(JsonValueKind.Object, document.RootElement.ValueKind);
        }
    }
}
