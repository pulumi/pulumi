// Copyright 2016-2019, Pulumi Corporation

using System.Text.Json;
using System.Threading.Tasks;
using Pulumi.Serialization;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class ArgsConverterTests : ConverterTests
    {
        public class SimpleInvokeArgs1 : InvokeArgs
        {
            [Input("s")]
            public string? S { get; set; }
        }

        public class ComplexInvokeArgs1 : InvokeArgs
        {
            [Input("v")]
            public SimpleInvokeArgs1? V { get; set; }
        }

        public class SimpleResourceArgs1 : ResourceArgs
        {
            [Input("s")]
            public Input<string>? S { get; set; }
        }

        public class ComplexResourceArgs1 : ResourceArgs
        {
            [Input("v")]
            public Input<SimpleResourceArgs1>? V { get; set; }
        }

        private async Task Test(object args, string expected)
        {
            var serialized = await SerializeToValueAsync(args);
            var converted = Converter.ConvertValue<JsonElement>("", serialized);
            var value = converted.Value.GetProperty("v").GetProperty("s").GetString();
            Assert.Equal(expected, value);
        }

        [Fact]
        public async Task InvokeArgs()
        {
            var args = new ComplexInvokeArgs1 { V = new SimpleInvokeArgs1 { S = "value1" } };
            await Test(args, "value1");
        }

        [Fact]
        public async Task ResourceArgs()
        {
            var args = new ComplexResourceArgs1 { V = new SimpleResourceArgs1 { S = "value2" } };
            await Test(args, "value2");
        }
    }
}
