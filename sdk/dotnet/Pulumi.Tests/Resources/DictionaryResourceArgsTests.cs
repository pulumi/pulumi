// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using Xunit;

namespace Pulumi.Tests.Resources
{
    public class DictionaryResourceArgsTests
    {
        [Fact]
        public void ConstructorValidValues()
        {
            var dict = new Dictionary<string, object?>
            {
                { "null", null },
                { "string", "foo" },
                { "int", 123 },
                { "dictionary", new Dictionary<string, object?> { {"foo", "bar"} }.ToImmutableDictionary() },
                { "output", Output.CreateSecret("secret") },
            };
            new DictionaryResourceArgs(dict.ToImmutableDictionary());
        }

        [Fact]
        public void ConstructorInvalidValue()
        {
            var dict = new Dictionary<string, object?>
            {
                { "arbitrary", new { Foo = "bar" } },
            };
            Assert.Throws<InvalidOperationException>(() =>
                new DictionaryResourceArgs(dict.ToImmutableDictionary()));
        }
    }
}
