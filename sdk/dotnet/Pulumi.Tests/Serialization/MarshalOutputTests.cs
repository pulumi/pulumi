// Copyright 2016-2021, Pulumi Corporation
using System.Collections.Generic;
using Pulumi.Serialization;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class MarshalTests : PulumiTest
    {
        private struct TestValue
        {
            object? value_;
            object? expected_;
            List<string> deps;
            ImmutableHashSet<Resource> resources;
            bool isKnown;
            bool isSecret;

            public TestValue(object? value_, object? expected, List<string> deps, bool isKnown, bool isSecret)
            {
                this.value_ = value_;
                this.expected_ = expected;
                this.deps = deps;
                this.isKnown = isKnown;
                this.isSecret = isSecret;
                var resources = new HashSet<Resource>();
                foreach (var d in deps)
                    resources.Add(new DependencyResource(d));
                this.resources = resources.ToImmutableHashSet();
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
                    // TODO we don't create the `Ouput` we want here.
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
                    if (deps.Count > 0) b.Add("dependencies", deps);
                    return b.ToImmutableDictionary();
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

        [Fact]
        static public async Task TransferProperties()
        {
            foreach (var test in TestValue.AllValues())
            {
                await RunInNormal(async () =>
                {
                    var s = new Serializer(excessiveDebugOutput: false, keepOutputValues: true);
                    var actual = await s.SerializeAsync("", test.input, true).ConfigureAwait(false);
                    Assert.Equal(test.expected, actual);
                });
            }
        }
    }
}
