// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using Pulumi.Automation.Serialization;
using Xunit;

namespace Pulumi.Automation.Tests.Serialization
{
    public class DynamicObjectTests
    {
        private static LocalSerializer Serializer = new LocalSerializer();

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

            var dict = Serializer.DeserializeYaml<Dictionary<string, object>>(yaml);
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

            var dict = Serializer.DeserializeJson<Dictionary<string, object>>(json);
            Console.WriteLine("ok");
        }
    }
}
