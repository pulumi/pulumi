// Copyright 2016-2021, Pulumi Corporation

using System.Collections.Generic;
using Pulumi.Automation.Serialization;
using Xunit;

namespace Pulumi.Automation.Tests.Serialization
{
    public class DynamicObjectTests
    {
        private static readonly LocalSerializer _serializer = new LocalSerializer();

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

            var dict = _serializer.DeserializeYaml<Dictionary<string, object>>(yaml);
            Assert.NotNull(dict);
            Assert.NotEmpty(dict);
            Assert.Equal(4, dict.Count);
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

            var dict = _serializer.DeserializeJson<Dictionary<string, object>>(json);
            Assert.NotNull(dict);
            Assert.NotEmpty(dict);
            Assert.Equal(4, dict.Count);
        }
    }
}
