using System;
using System.Collections.Generic;
using System.Text;
using Newtonsoft.Json;
using Pulumi.X.Automation;
using Pulumi.X.Automation.Serialization;
using Xunit;

namespace Pulumi.Tests.X.Automation
{
    public class ProjectRuntimeYamlConverterTests
    {
        private static LocalSerializer Serializer = new LocalSerializer();

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

            var model = Serializer.DeserializeYaml<ProjectSettingsModel>(yaml);
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

            var model = Serializer.DeserializeYaml<ProjectSettingsModel>(yaml);
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

            var yaml = Serializer.SerializeYaml(runtime);
            Console.WriteLine(yaml);

            Assert.Equal("dotnet\r\n", yaml);
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

            var yaml = Serializer.SerializeYaml(runtime);
            Console.WriteLine(yaml);

            var expected = new StringBuilder();
            expected.Append("name: dotnet\r\n");
            expected.Append("options:\r\n");
            expected.Append("  typescript: true\r\n");

            Assert.Equal(expected.ToString(), yaml);
        }

        [Fact]
        public void Dynamic_With_YamlDotNet()
        {
            const string yaml = @"
one: 123
two: two
three: true
nested:
  test: test
  testtwo: 123
";

            var dict = Serializer.DeserializeYaml<IDictionary<string, object>>(yaml);
            Console.WriteLine("ok");
        }

        [Fact]
        public void Dynamic_With_NewtonsoftJson()
        {
            const string json = @"
{
    ""one"": 123,
    ""two"": ""two"",
    ""three"": true,
    ""four"": 3.14,
    ""nested"": {
        ""test"": ""test"",
        ""testtwo"": 123,
    }
}
";

            var dict = JsonConvert.DeserializeObject<IDictionary<string, object>>(json);
            Console.WriteLine("ok");
        }

        [Fact]
        public void Dynamic_With_SystemTextJson()
        {
            const string json = @"
{
    ""one"": 123,
    ""two"": ""two"",
    ""three"": true,
    ""nested"": {
        ""test"": ""test"",
        ""testtwo"": 123,
    }
}
";

            var dict = Serializer.DeserializeJson<IDictionary<string, object>>(json);
            Console.WriteLine("ok");
        }
    }
}
