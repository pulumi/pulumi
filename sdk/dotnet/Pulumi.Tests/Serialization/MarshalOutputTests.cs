// Copyright 2016-2021, Pulumi Corporation
using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using Pulumi.Serialization;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class MarshalTests : PulumiTest
    {
        public struct TestValue
        {
            public readonly string Name;
            public ImmutableDictionary<string, object?> Expected;

            public Output<object?> Input;
            internal OutputData<object?> ExpectedRoundTrip;

            public TestValue(object? value_, object? expected, string[] deps, bool isKnown, bool isSecret)
            {
                Name = $"Output(deps={deps}, value={value_}, isKnown={isKnown}, isSecret={isSecret})";
                var r = new HashSet<Resource>();
                foreach (var d in deps)
                    r.Add(new DependencyResource(d));
                var resources = r.ToImmutableHashSet();


                var b = ImmutableDictionary.CreateBuilder<string, object?>();
                b.Add(Constants.SpecialSigKey, Constants.SpecialOutputValueSig);
                if (isKnown) b.Add("value", expected);
                if (isSecret) b.Add("secret", isSecret);
                if (deps.Length > 0) b.Add("dependencies", deps);
                Expected = b.ToImmutableDictionary();

                var data = OutputData.Create<object?>(resources, value_, isKnown, isSecret);
                Input = new Output<object?>(Task.FromResult(data));

                ExpectedRoundTrip = OutputData.Create<object?>(resources, isKnown ? ToValue(expected) : null, isKnown, isSecret);
            }

            public override string ToString() => Name;
        }

        public static IEnumerable<object[]> AllValues =>
            from tv in new object?[]
            {
                null,
                0,
                1,
                "",
                "hi",
                ImmutableDictionary.CreateBuilder<string, object?>().ToImmutable(),
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

        /// <summary>
        /// Asserts that two <c>OutputData<T></c> instances are sufficiently equivalent.
        /// </summary>
        private async static Task AssertEquivalent<T>(OutputData<T> e, OutputData<T> a)
        {
            System.Func<IEnumerable<Resource>, Task<ImmutableSortedSet<string>>> urns = async (resources) =>
            {
                var s = ImmutableSortedSet.CreateBuilder<string>();
                foreach (var r in resources)
                {
                    s.Add((await r.Urn.DataTask).Value);
                }
                return s.ToImmutable();
            };
            Assert.Equal(e.IsSecret, a.IsSecret);
            Assert.Equal(e.IsKnown, a.IsKnown);
            Assert.Equal(e.Value, a.Value);
            Assert.Equal(await urns(e.Resources), await urns(a.Resources));
        }

        /// <summary>
        /// This is a poor implementation of the <c>ToValue</c> function, designed only for test code.
        /// </summary>
        private static Value ToValue(object? o)
        {
            switch (o)
            {
                case null:
                    return new Value { NullValue = NullValue.NullValue };
                case string str:
                    return new Value { StringValue = str };
                case int i:
                    return new Value { NumberValue = i };
                case ImmutableDictionary<string, object?> dict:
                    var s = new Struct();
                    foreach (var (k, v) in dict)
                    {
                        s.Fields.Add(k, ToValue(v));
                    }
                    return new Value { StructValue = s };
                case bool b:
                    return new Value { BoolValue = b };
                case List<object> l:
                    return ToValue(l.ToImmutableArray());
                case ImmutableArray<object> iArray:
                    var list = new ListValue();
                    foreach (var v in iArray)
                        list.Values.Add(ToValue(v));
                    return new Value { ListValue = list };
                default:
                    throw new System.TypeAccessException($"Failed to create value type of type {o.GetType().FullName}");
            }
        }

        [Theory]
        [MemberData(nameof(AllValues))]
        static public async Task TransferProperties(TestValue test)
        {
            await RunInNormal(async () =>
            {
                var s = new Serializer(excessiveDebugOutput: false);
                var actual = await s.SerializeAsync(
                    "", test.Input,
                    keepResources: true,
                    keepOutputValues: true).ConfigureAwait(false) as ImmutableDictionary<string, object>;
                AssertEquivalent(test.Expected, actual!);
                var back = Deserializer.Deserialize(ToValue(actual!));
                await AssertEquivalent(test.ExpectedRoundTrip, back);
            });
        }
    }
}
