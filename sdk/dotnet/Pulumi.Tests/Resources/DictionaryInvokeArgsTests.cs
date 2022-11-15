// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using Xunit;

namespace Pulumi.Tests.Resources
{
    public class DictionaryInvokeArgsTests
    {
        [Fact]
        public void ConstructorValidValues()
        {
            try
            {
                var dict = new Dictionary<string, object?>
                {
                    { "null", null },
                    { "string", "foo" },
                    { "int", 123 },
                    { "dictionary", new Dictionary<string, object?> { {"foo", "bar"} }.ToImmutableDictionary() },
                    { "output", Output.CreateSecret("secret") },
                };
                var dictionary = new DictionaryInvokeArgs(dict.ToImmutableDictionary());
            }
            catch 
            {
                Assert.True(false, "DictionaryInvokeArgs constructor should not throw an exception");
            }
        }

        [Fact]
        public void ConstructorInvalidValue()
        {
            var dict = new Dictionary<string, object?>
            {
                { "arbitrary", new { Foo = "bar" } },
            };

            Assert.Throws<InvalidOperationException>(() =>
            {
                new DictionaryInvokeArgs(dict.ToImmutableDictionary());
            });
        }
    }
}
