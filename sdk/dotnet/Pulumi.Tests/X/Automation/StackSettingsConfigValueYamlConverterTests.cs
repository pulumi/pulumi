using System.Collections.Generic;
using System.Text;
using Pulumi.X.Automation;
using Pulumi.X.Automation.Serialization;
using Xunit;

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
            Assert.Equal("plain", value.ValueString);
            Assert.Null(value.ValueObject);
            Assert.False(value.IsSecure);
            Assert.False(value.IsObject);
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
            Assert.Equal("secret", value.ValueString);
            Assert.Null(value.ValueObject);
            Assert.True(value.IsSecure);
            Assert.False(value.IsObject);
        }

        [Fact]
        public void CanDeserializeObject()
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

            var settings = Serializer.DeserializeYaml<StackSettings>(yaml);
            Assert.NotNull(settings?.Config);
            Assert.True(settings!.Config!.ContainsKey("value"));

            var value = settings.Config["value"];
            Assert.NotNull(value);
            Assert.Null(value.ValueString);
            Assert.NotNull(value.ValueObject);
            Assert.False(value.IsSecure);
            Assert.True(value.IsObject);

            Assert.True(value.ValueObject!.ContainsKey("test"));
            var testProperty = value.ValueObject["test"];
            Assert.NotNull(testProperty);
            Assert.IsType<string>(testProperty);
            Assert.Equal("test", testProperty);

            Assert.True(value.ValueObject.ContainsKey("nested"));
            var nestedProperty = value.ValueObject["nested"];
            Assert.NotNull(nestedProperty);
            Assert.IsType<Dictionary<string, object>>(nestedProperty);
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

        [Fact]
        public void SerializesObject()
        {
            var dictionary = new Dictionary<string, object>()
            {
                ["test"] = "test",
                ["nested"] = new
                {
                    one = 1,
                    two = true,
                    three = "three",
                },
            };

            var value = new StackSettingsConfigValue(dictionary);
            var yaml = Serializer.SerializeYaml(value);

            var expected = new StringBuilder();
            expected.Append("test: test\r\n");
            expected.Append("nested:\r\n");
            expected.Append("  one: 1\r\n");
            expected.Append("  two: true\r\n");
            expected.Append("  three: three\r\n");

            Assert.Equal(expected.ToString(), yaml);
        }
    }
}
