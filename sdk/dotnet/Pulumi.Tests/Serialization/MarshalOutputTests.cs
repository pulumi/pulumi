// Copyright 2016-2021, Pulumi Corporation
using System.Collections.Generic;
using Pulumi.Serialization;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class MarshalTests : PulumiTest
    {
        private struct TestValue
        {
            object? value_;
            object? expected_;
            string[] deps;
            ImmutableHashSet<Resource> resources;
            bool isKnown;
            bool isSecret;

            public TestValue(object? value_, object? expected, List<string> deps, bool isKnown, bool isSecret)
            {
                this.value_ = value_;
                this.expected_ = expected;
                this.deps = deps.ToArray();
                this.isKnown = isKnown;
                this.isSecret = isSecret;
                var r = new HashSet<Resource>();
                foreach (var d in deps)
                    r.Add(new DependencyResource(d));
                this.resources = r.ToImmutableHashSet();
            }

            private static List<(object?, object?)> testValues => new List<(object?, object?)>{
                (null, null),
                (0, 0),
                (1, 1),
                ("", ""),
                ("hi", "hi"),
                (ImmutableDictionary.CreateBuilder<string, object?>().ToImmutable(),
                 ImmutableDictionary.CreateBuilder<string, object?>().ToImmutable()),
                (new List<object?>(), new List<object?>()),
                };

            public string name => $"Output(deps={deps}, value={value_}, isKnown={isKnown}, isSecret={isSecret})";
            public Output<object?> input
            {
                get
                {
                    var d = OutputData.Create<object?>(this.resources, value_, isKnown, isSecret);
                    return new Output<object?>(Task.FromResult(d));
                }
            }

            public ImmutableDictionary<string, object?> expected
            {
                get
                {
                    var b = ImmutableDictionary.CreateBuilder<string, object?>();
                    b.Add(Constants.SpecialSigKey, Constants.SpecialOutputValueSig);
                    if (isKnown) b.Add("value", expected_);
                    if (isSecret) b.Add("secret", isSecret);
                    if (deps.Length > 0) b.Add("dependencies", deps);
                    return b.ToImmutableDictionary();
                }
            }

            public Output<object?> expectedRoundTrip
            {
                get
                {
                    var d = OutputData.Create<object?>(this.resources, isKnown ? this.expected : null, isKnown, isSecret);
                    return new Output<object?>(Task.FromResult(d));
                }
            }

            public static IEnumerable<TestValue> AllValues()
            {
                var result = new List<TestValue>();
                foreach (var tv in testValues)
                    foreach (var deps in new List<List<string>>
                    { new List<string>(), new List<string> { "fakeURN1", "fakeURN2" } })
                        foreach (var isSecret in new List<bool> { true, false })
                            foreach (var isKnown in new List<bool> { true, false })
                                result.Add(new TestValue(tv.Item1, tv.Item2, deps, isSecret, isKnown));
                return result;
            }
        }

        /// <summary>
        /// Asserts that two dictionaries are sufficiently equivalent.
        /// </summary>
        private static void ShouldBeEquivalent(
            in ImmutableDictionary<string, object?> expected,
            in ImmutableDictionary<string, object?> actual)
        {
            var expectedKeys = expected.Keys.ToImmutableHashSet();
            var actualKeys = actual.Keys.ToImmutableHashSet();
            Assert.True(expectedKeys.SetEquals(actualKeys), "Key mismatch");
            foreach (var k in expectedKeys)
            {
                var expectedArray = expected[k] as object[];
                if (expectedArray != null)
                {
                    var actualArray = (actual[k] as object[])!;
                    System.Array.Sort(actualArray);
                    System.Array.Sort(expectedArray);

                    Assert.Equal(expected, actual);
                }
                else
                {
                    Assert.Equal(expected[k], actual[k]);
                }
            }
        }
        private static void ShouldBeEquivalent<T>(OutputData<T> e, OutputData<T> a)
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
            Assert.Equal(urns(e.Resources), urns(a.Resources));
        }

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
                default:
                    throw new System.TypeAccessException($"Failed to create value type of type {o.GetType().FullName}");
            }
        }

        [Fact]
        static public async Task TransferProperties()
        {
            foreach (var test in TestValue.AllValues())
            {
                await RunInNormal(async () =>
                {
                    var s = new Serializer(excessiveDebugOutput: false, keepOutputValues: true);
                    var actual = await s.SerializeAsync("", test.input, true).ConfigureAwait(false) as ImmutableDictionary<string, object>;
                    ShouldBeEquivalent(test.expected, actual!);
                    var f = new Struct();
                    foreach (var (k, v) in actual!)
                    {
                        f.Fields.Add(k, ToValue(v));
                    }
                    var value = new Value();
                    value.StructValue = f;
                    var back = Deserializer.Deserialize(value);
                    ShouldBeEquivalent(await test.expectedRoundTrip.DataTask, back);
                });
            }
        }
    }
}
