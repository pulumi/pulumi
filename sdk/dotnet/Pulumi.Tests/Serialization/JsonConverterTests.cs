// Copyright 2016-2019, Pulumi Corporation

using System.Text.Json;
using System.Threading.Tasks;
using Pulumi.Serialization;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class JsonConverterTests : ConverterTests
    {
        private async Task Test(string json, string expected)
        {
            var element = JsonDocument.Parse(json).RootElement;
            var serialized = await SerializeToValueAsync(element);
            var converted = Converter.ConvertValue<JsonElement>("", serialized);

            Assert.Equal(expected, converted.Value.ToString());
        }

        [Fact]
        public async Task TestString()
        {
            await Test("\"x\"", "x");
        }

        [Fact]
        public async Task TestNumber()
        {
            await Test("1.1", "1.1");
        }

        [Fact]
        public async Task TestBoolean()
        {
            await Test("true", "True");
        }

        [Fact]
        public async Task TestNull()
        {
            await Test("null", "");
        }

        [Fact]
        public async Task TestArray()
        {
            await Test("[1, true, null]", "[1,true,null]");
        }

        [Fact]
        public async Task TestObject()
        {
            await Test("{ \"n\": 1 }", "{\"n\":1}");
        }
    }
}
