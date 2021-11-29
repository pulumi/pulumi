// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading.Tasks;
using Pulumi.Serialization;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class MarshalOutputTests : PulumiTest
    {
        public static IEnumerable<object[]> BasicSerializeData =>
            from value in new object?[]
            {
                null,
                0.0,
                1.0,
                "",
                "hi",
                ImmutableDictionary<string, object?>.Empty,
                ImmutableArray<object?>.Empty,
            }
            from deps in new[] { Array.Empty<string>(), new[] { "fakeURN1", "fakeURN2" } }
            from isKnown in new[] { true, false }
            from isSecret in new[] { true, false }
            select new object[] { value, deps, isKnown, isSecret };

        [Theory]
        [MemberData(nameof(BasicSerializeData))]
        public static Task TestBasicSerialize(object? value, string[] deps, bool isKnown, bool isSecret) => RunInNormal(async () =>
        {
            var resources = ImmutableHashSet.CreateRange<Resource>(deps.Select(d => new DependencyResource(d)));
            var data = OutputData.Create(resources, value, isKnown, isSecret);
            var input = new Output<object?>(Task.FromResult(data));

            var expected = isKnown && !isSecret && deps.Length == 0
                ? value
                : CreateOutputValue(value, isKnown, isSecret, deps);

            var s = new Serializer(excessiveDebugOutput: false);
            var actual = await s.SerializeAsync("", input, keepResources: true, keepOutputValues: true);
            Assert.Equal(expected, actual);
        });

        public sealed class FooArgs : ResourceArgs
        {
            [Input("foo")]
            public Input<string>? Foo { get; set; }
        }

        public sealed class BarArgs : ResourceArgs
        {
            [Input("foo")]
            public Input<FooArgs>? Foo { get; set; }
        }

        public static IEnumerable<object[]> SerializeData() => new object[][]
        {
            new object[]
            {
                new FooArgs { Foo = "hello" },
                ImmutableDictionary<string, object>.Empty.Add("foo", "hello")
            },
            new object[]
            {
                new FooArgs { Foo = Output.Create("hello") },
                ImmutableDictionary<string, object>.Empty.Add("foo", "hello")
            },
            new object[]
            {
                new FooArgs { Foo = Output.CreateSecret("hello") },
                ImmutableDictionary<string, object>.Empty.Add("foo", CreateOutputValue("hello", isSecret: true))
            },
            new object[]
            {
                new List<Input<string>> { "hello" },
                ImmutableArray<object>.Empty.Add("hello")
            },
            new object[]
            {
                new List<Input<string>> { Output.Create("hello") },
                ImmutableArray<object>.Empty.Add("hello")
            },
            new object[]
            {
                new List<Input<string>> { Output.CreateSecret("hello") },
                ImmutableArray<object>.Empty.Add(CreateOutputValue("hello", isSecret: true))
            },
            new object[]
            {
                new Dictionary<string, Input<string>> { { "foo", "hello" } },
                ImmutableDictionary<string, object>.Empty.Add("foo", "hello")
            },
            new object[]
            {
                new Dictionary<string, Input<string>> { { "foo", Output.Create("hello") } },
                ImmutableDictionary<string, object>.Empty.Add("foo", "hello")
            },
            new object[]
            {
                new Dictionary<string, Input<string>> { { "foo", Output.CreateSecret("hello") } },
                ImmutableDictionary<string, object>.Empty.Add("foo", CreateOutputValue("hello", isSecret: true))
            },
            new object[]
            {
                new BarArgs { Foo = new FooArgs { Foo = "hello" } },
                ImmutableDictionary<string, object>.Empty.Add("foo",
                    ImmutableDictionary<string, object>.Empty.Add("foo", "hello"))
            },
            new object[]
            {
                new BarArgs { Foo = new FooArgs { Foo = Output.Create("hello") } },
                ImmutableDictionary<string, object>.Empty.Add("foo",
                    ImmutableDictionary<string, object>.Empty.Add("foo", "hello"))
            },
            // Repro #8474
            UnknownDefaultValue<ImmutableArray<bool>>(),
            UnknownDefaultValue<ImmutableArray<object>>(),
            UnknownDefaultValue<ImmutableDictionary<string,bool>>(),
        };

        // Ensure that we can safely serialize unknown values. This causes issues with values whose
        // defaults are not safe to interact with (ImmutableArray<T> for example).
        private static object[] UnknownDefaultValue<T>()
            where T : notnull
        {
            T inner = default;
            var outputdata = OutputData.Create(ImmutableHashSet<Resource>.Empty, inner!, isKnown: false, isSecret: false);
            var output = new Output<T>(Task.FromResult(outputdata));
            return new object[]
                {
                    output,
                 ImmutableDictionary<string, object>.Empty.Add(Constants.SpecialSigKey, Constants.SpecialOutputValueSig),
                };
        }

        [Theory]
        [MemberData(nameof(SerializeData))]
        public static Task TestSerialize(object input, object expected) => RunInNormal(async () =>
        {
            var s = new Serializer(excessiveDebugOutput: false);
            var actual = await s.SerializeAsync("", input, keepResources: true, keepOutputValues: true);
            Assert.Equal(expected, actual);
        });

        private static ImmutableDictionary<string, object?> CreateOutputValue(
            object? value, bool isKnown = true, bool isSecret = false, params string[] deps)
        {
            var b = ImmutableDictionary.CreateBuilder<string, object?>();
            b.Add(Constants.SpecialSigKey, Constants.SpecialOutputValueSig);
            if (isKnown) b.Add(Constants.ValueName, value);
            if (isSecret) b.Add(Constants.SecretName, isSecret);
            if (deps.Length > 0) b.Add(Constants.DependenciesName, deps.ToImmutableArray());
            return b.ToImmutableDictionary();
        }
    }
}
