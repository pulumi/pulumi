using System.Collections.Generic;
using System.Text.Json;
using Pulumi.X.Automation;
using Pulumi.X.Automation.Serialization;
using Xunit;

namespace Pulumi.Tests.X.Automation
{
    public class StackSettingsConfigValueJsonConverterTests
    {
        private static LocalSerializer Serializer = new LocalSerializer();

        [Fact]
        public void CanDeserializePlainString()
        {
            const string json = @"
{
    ""config"": {
        ""test"": ""plain""
    }  
}
";

            var settings = Serializer.DeserializeJson<StackSettings>(json);
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
            const string json = @"
{
    ""config"": {
        ""test"": {
            ""secure"": ""secret""
        }
    }  
}
";

            var settings = Serializer.DeserializeJson<StackSettings>(json);
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
            const string json = @"
{
    ""config"": {
        ""value"": {
            ""test"": ""test"",
            ""nested"": {
                ""one"": 1,
                ""two"": true,
                ""three"": ""three""
            }
        }
    } 
}
";

            var settings = Serializer.DeserializeJson<StackSettings>(json);
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
            Assert.IsType<JsonElement>(testProperty);
            var testJsonElement = (JsonElement)testProperty;
            Assert.Equal(JsonValueKind.String, testJsonElement.ValueKind);
            Assert.Equal("test", testJsonElement.GetString());

            Assert.True(value.ValueObject.ContainsKey("nested"));
            var nestedProperty = value.ValueObject["nested"];
            Assert.NotNull(nestedProperty);
            Assert.IsType<JsonElement>(nestedProperty);
            var nestedJsonElement = (JsonElement)nestedProperty;
            Assert.Equal(JsonValueKind.Object, nestedJsonElement.ValueKind);
        }

        [Fact]
        public void SerializesPlainStringAsString()
        {
            var value = new StackSettingsConfigValue("test", false);
            var json = Serializer.SerializeJson(value);

            var element = JsonSerializer.Deserialize<JsonElement>(json);
            Assert.Equal(JsonValueKind.String, element.ValueKind);
            Assert.Equal("test", element.GetString());
        }

        [Fact]
        public void SerializesSecureStringAsObject()
        {
            var value = new StackSettingsConfigValue("secret", true);
            var json = Serializer.SerializeJson(value);

            var element = JsonSerializer.Deserialize<JsonElement>(json);
            Assert.Equal(JsonValueKind.Object, element.ValueKind);
            Assert.True(element.TryGetProperty("secure", out var secureProperty));
            Assert.Equal(JsonValueKind.String, secureProperty.ValueKind);
            Assert.Equal("secret", secureProperty.GetString());
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
            var json = Serializer.SerializeJson(value);

            var element = JsonSerializer.Deserialize<JsonElement>(json);
            Assert.Equal(JsonValueKind.Object, element.ValueKind);

            Assert.True(element.TryGetProperty("test", out var testProperty));
            Assert.Equal(JsonValueKind.String, testProperty.ValueKind);
            Assert.Equal("test", testProperty.GetString());

            Assert.True(element.TryGetProperty("nested", out var nestedProperty));
            Assert.Equal(JsonValueKind.Object, nestedProperty.ValueKind);
        }
    }
}
