using System;
using System.Collections.Generic;
using System.Text;
using Pulumi.X.Automation;
using Pulumi.X.Automation.Serialization;
using Xunit;
using YamlDotNet.Core;

namespace Pulumi.Tests.X.Automation
{
    public class StackSettingsConfigValueYamlConverterTests
    {
        private static LocalSerializer Serializer = new LocalSerializer();

        [Fact]
        public void CanDeserializePlainString()
        {
            const string yaml = @"
config:
  test: plain
";

            var settings = Serializer.DeserializeYaml<StackSettings>(yaml);
            Assert.NotNull(settings?.Config);
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

            var settings = Serializer.DeserializeYaml<StackSettings>(yaml);
            Assert.NotNull(settings?.Config);
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
                () => Serializer.DeserializeYaml<StackSettings>(yaml));
        }

        [Fact]
        public void SerializesPlainStringAsString()
        {
            var value = new StackSettingsConfigValue("test", false);
            var yaml = Serializer.SerializeYaml(value);
            Assert.Equal("test\r\n", yaml);
        }

        [Fact]
        public void SerializesSecureStringAsObject()
        {
            var value = new StackSettingsConfigValue("secret", true);
            var yaml = Serializer.SerializeYaml(value);
            Assert.Equal("secure: secret\r\n", yaml);
        }
    }
}
