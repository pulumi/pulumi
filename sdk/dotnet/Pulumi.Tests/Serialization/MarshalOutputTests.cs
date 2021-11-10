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
        public struct TestValue
        {
            public readonly string Name;
            public readonly ImmutableDictionary<string, object?> Expected;
            public readonly Output<object?> Input;

            public TestValue(object? value, object? expected, string[] deps, bool isKnown, bool isSecret)
            {
                Name = $"Output(deps={deps}, value={value}, isKnown={isKnown}, isSecret={isSecret})";
                var resources = ImmutableHashSet.CreateRange<Resource>(deps.Select(d => new DependencyResource(d)));

                var b = ImmutableDictionary.CreateBuilder<string, object?>();
                b.Add(Constants.SpecialSigKey, Constants.SpecialOutputValueSig);
                if (isKnown) b.Add(Constants.ValueName, expected);
                if (isSecret) b.Add(Constants.SecretName, isSecret);
                if (deps.Length > 0) b.Add(Constants.DependenciesName, deps.ToImmutableArray());
                Expected = b.ToImmutableDictionary();

                var data = OutputData.Create(resources, value, isKnown, isSecret);
                Input = new Output<object?>(Task.FromResult(data));
            }

            public override string ToString() => Name;
        }

        public static IEnumerable<object[]> AllValues =>
            from tv in new object?[]
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
            from isSecret in new List<bool> { true, false }
            from isKnown in new List<bool> { true, false }
            select new object[] { new TestValue(tv, tv, deps, isKnown, isSecret) };

        [Theory]
        [MemberData(nameof(AllValues))]
        public static Task TestSerialize(TestValue test)
            => RunInNormal(async () =>
            {
                var s = new Serializer(excessiveDebugOutput: false);
                var actual = await s.SerializeAsync("", test.Input, keepResources: true, keepOutputValues: true);
                Assert.Equal(test.Expected, actual!);
            });
    }
}
