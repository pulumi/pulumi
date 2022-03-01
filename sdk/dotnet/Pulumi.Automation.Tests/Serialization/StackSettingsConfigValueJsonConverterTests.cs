// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Text.Json;
using Pulumi.Automation.Serialization;
using Xunit;

namespace Pulumi.Automation.Tests.Serialization
{
    public class StackSettingsConfigValueJsonConverterTests
    {
        private static readonly LocalSerializer _serializer = new LocalSerializer();

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

            var settings = _serializer.DeserializeJson<StackSettings>(json);
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
            const string json = @"
{
    ""config"": {
        ""test"": {
            ""secure"": ""secret""
        }
    }  
}
";

            var settings = _serializer.DeserializeJson<StackSettings>(json);
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

            Assert.Throws<NotSupportedException>(
                () => _serializer.DeserializeJson<StackSettings>(json));
        }

        [Fact]
        public void SerializesPlainStringAsString()
        {
            var value = new StackSettingsConfigValue("test", false);
            var json = _serializer.SerializeJson(value);

            var element = JsonSerializer.Deserialize<JsonElement>(json);
            Assert.Equal(JsonValueKind.String, element.ValueKind);
            Assert.Equal("test", element.GetString());
        }

        [Fact]
        public void SerializesSecureStringAsObject()
        {
            var value = new StackSettingsConfigValue("secret", true);
            var json = _serializer.SerializeJson(value);

            var element = JsonSerializer.Deserialize<JsonElement>(json);
            Assert.Equal(JsonValueKind.Object, element.ValueKind);
            Assert.True(element.TryGetProperty("secure", out var secureProperty));
            Assert.Equal(JsonValueKind.String, secureProperty.ValueKind);
            Assert.Equal("secret", secureProperty.GetString());
        }
    }
}
