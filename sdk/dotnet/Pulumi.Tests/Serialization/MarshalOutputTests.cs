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
                if (deps.Length > 0) b.Add(Constants.DependenciesName, deps);
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
                new List<object?>(),
            }
            from deps in new[] { Array.Empty<string>(), new[] { "fakeURN1", "fakeURN2" } }
            from isSecret in new List<bool> { true, false }
            from isKnown in new List<bool> { true, false }
            select new object[] { new TestValue(tv, tv, deps, isSecret, isKnown) };

        /// <summary>
        /// Asserts that two dictionaries are sufficiently equivalent.
        /// </summary>
        private static void AssertEquivalent(
            in ImmutableDictionary<string, object?> expected,
            in ImmutableDictionary<string, object?> actual)
        {
            AssertEx.Equivalent(expected.Keys, actual.Keys);
            foreach (var (key, expectedValue) in expected)
            {
                var actualValue = actual[key];
                if (expectedValue is IEnumerable<object> expectedCollection)
                {
                    Assert.IsAssignableFrom<IEnumerable<object>>(actualValue);
                    AssertEx.Equivalent(expectedCollection, (IEnumerable<object>)actualValue!);
                }
                else
                {
                    Assert.Equal(expectedValue, actualValue);
                }
            }
        }

        [Theory]
        [MemberData(nameof(AllValues))]
        public static Task TestSerialize(TestValue test)
            => RunInNormal(async () =>
            {
                var s = new Serializer(excessiveDebugOutput: false);
                var actual = await s.SerializeAsync(
                    "", test.Input,
                    keepResources: true,
                    keepOutputValues: true).ConfigureAwait(false) as ImmutableDictionary<string, object>;
                AssertEquivalent(test.Expected, actual!);
            });
    }
}
