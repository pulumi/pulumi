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
        public void DeserializeObjectWorks()
        {
            const string json = @"
{
    ""config"": {
        ""value"": {
            ""hello"": ""world""
        }
    } 
}
";

            var settings = _serializer.DeserializeJson<StackSettings>(json);
            Assert.NotNull(settings.Config);
            Assert.True(settings!.Config!.ContainsKey("value"));

            var value = settings.Config["value"];
            Assert.NotNull(value);
            Assert.Equal("{\"hello\":\"world\"}", value.Value);
            Assert.False(value.IsSecure);
        }

        [Fact]
        public void DeserializeArrayWorks()
        {
            const string json = @"
{
    ""config"": {
        ""value"": [1,2,3,4,5]
    } 
}
";

            var settings = _serializer.DeserializeJson<StackSettings>(json);
            Assert.NotNull(settings.Config);
            Assert.True(settings!.Config!.ContainsKey("value"));

            var value = settings.Config["value"];
            Assert.NotNull(value);
            Assert.Equal("[1,2,3,4,5]", value.Value);
            Assert.False(value.IsSecure);
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
