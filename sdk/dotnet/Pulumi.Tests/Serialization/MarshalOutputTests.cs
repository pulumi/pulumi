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

            var b = ImmutableDictionary.CreateBuilder<string, object?>();
            b.Add(Constants.SpecialSigKey, Constants.SpecialOutputValueSig);
            if (isKnown) b.Add(Constants.ValueName, value);
            if (isSecret) b.Add(Constants.SecretName, isSecret);
            if (deps.Length > 0) b.Add(Constants.DependenciesName, deps.ToImmutableArray());
            var expected = b.ToImmutableDictionary();

            var s = new Serializer(excessiveDebugOutput: false);
            var actual = await s.SerializeAsync("", input, keepResources: true, keepOutputValues: true);
            Assert.Equal(expected, actual);
        });
    }
}
