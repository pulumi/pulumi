// Copyright 2016-2021, Pulumi Corporation

using System;
using Pulumi.Automation.Serialization;
using Xunit;
using YamlDotNet.Core;

namespace Pulumi.Automation.Tests.Serialization
{
    public class StackSettingsConfigValueYamlConverterTests
    {
        private static readonly LocalSerializer _serializer = new LocalSerializer();

        [Fact]
        public void CanDeserializePlainString()
        {
            const string yaml = @"
config:
  test: plain
";

            var settings = _serializer.DeserializeYaml<StackSettings>(yaml);
            Assert.NotNull(settings.Config);
            Assert.True(settings!.Config!.ContainsKey("test"));

            var value = settings.Config["test"];
            Assert.NotNull(value);
            Assert.Equal("plain", value.Value);
            Assert.False(value.IsSecure);
        }

        [Fact]
        public void CanDeserializeSecureString()
        {
            const string yaml = @"
config:
  test:
    secure: secret
";

            var settings = _serializer.DeserializeYaml<StackSettings>(yaml);
            Assert.NotNull(settings.Config);
            Assert.True(settings!.Config!.ContainsKey("test"));

            var value = settings.Config["test"];
            Assert.NotNull(value);
            Assert.Equal("secret", value.Value);
            Assert.True(value.IsSecure);
        }

        [Fact]
        public void CannotDeserializeObject()
        {
            const string yaml = @"
config:
  value:
    test: test
    nested:
      one: 1
      two: true
      three: three
";

            Assert.Throws<YamlException>(
                () => _serializer.DeserializeYaml<StackSettings>(yaml));
        }

        [Fact]
        public void SerializesPlainStringAsString()
        {
            var value = new StackSettingsConfigValue("test", false);
            var yaml = _serializer.SerializeYaml(value);
            Assert.Equal("test" + Environment.NewLine, yaml);
        }

        [Fact]
        public void SerializesSecureStringAsObject()
        {
            var value = new StackSettingsConfigValue("secret", true);
            var yaml = _serializer.SerializeYaml(value);
            Assert.Equal("secure: secret" + Environment.NewLine, yaml);
        }
    }
}
