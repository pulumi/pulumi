// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Xunit;

namespace Pulumi.Tests.Core
{
    public class InputTests : PulumiTest
    {
        [Fact]
        public Task MergeInputMaps()
            => RunInPreview(async () =>
            {
                var map1 = new InputMap<string>
                {
                    { "K1", "V1" },
                    { "K2", Output.Create("V2") },
                    { "K3", Output.Create("V3_wrong") }
                };

                var map2 = new InputMap<string>
                {
                    { "K3", Output.Create("V3") },
                    { "K4", "V4" }
                };

                var result = InputMap<string>.Merge(map1, map2);

                // Check the merged map
                var data = await result.ToOutput().DataTask.ConfigureAwait(false);
                Assert.True(data.IsKnown);
                Assert.Equal(4, data.Value.Count);
                for (var i = 1; i <=4; i++)
                    Assert.True(data.Value.Contains($"K{i}", $"V{i}"));

                // Check that the input maps haven't changed
                var map1Data = await map1.ToOutput().DataTask.ConfigureAwait(false);
                Assert.Equal(3, map1Data.Value.Count);
                Assert.True(map1Data.Value.ContainsValue("V3_wrong"));

                var map2Data = await map2.ToOutput().DataTask.ConfigureAwait(false);
                Assert.Equal(2, map2Data.Value.Count);
                Assert.True(map2Data.Value.ContainsValue("V3"));
            });

        [Fact]
        public Task InputMapCollectionInitializers()
            => RunInPreview(async () =>
            {
                var map = new InputMap<string>
                {
                    { "K1", "V1" },
                    { "K2", Output.Create("V2") },
                    new Dictionary<string, string> { { "K3", "V3" }, { "K4", "V4"} },
                    Output.Create(new Dictionary<string, string> { ["K5"] = "V5", ["K6"] = "V6" }.ToImmutableDictionary())
                };
                var data = await map.ToOutput().DataTask.ConfigureAwait(false);
                Assert.Equal(6, data.Value.Count);
                Assert.Equal(new Dictionary<string, string> { ["K1"] = "V1", ["K2"] = "V2", ["K3"] = "V3", ["K4"] = "V4", ["K5"] = "V5", ["K6"] = "V6" }, data.Value);
            });

        [Fact]
        public Task InputMapUnionInitializer()
            => RunInPreview(async () =>
            {
                var sample = new SampleArgs
                {
                    Dict =
                    {
                        { "left", "testValue" },
                        { "right", 123 },
                        { "t0", Union<string, int>.FromT0("left") },
                        { "t1", Union<string, int>.FromT1(456) },
                    }
                };
                var data = await sample.Dict.ToOutput().DataTask.ConfigureAwait(false);
                Assert.Equal(4, data.Value.Count);
                Assert.True(data.Value.ContainsValue("testValue"));
                Assert.True(data.Value.ContainsValue(123));
                Assert.True(data.Value.ContainsValue("left"));
                Assert.True(data.Value.ContainsValue(456));
            });

        [Fact]
        public Task InputListCollectionInitializers()
            => RunInPreview(async () =>
            {
                var list = new InputList<string>
                {
                    "V1",
                    Output.Create("V2"),
                    new[] { "V3", "V4" },
                    new List<string> { "V5", "V6" },
                    Output.Create(ImmutableArray.Create("V7", "V8"))
                };
                var data = await list.ToOutput().DataTask.ConfigureAwait(false);
                Assert.Equal(8, data.Value.Length);
                Assert.Equal(new[] { "V1", "V2", "V3", "V4", "V5", "V6", "V7", "V8" }, data.Value);
            });

        [Fact]
        public Task InputListUnionInitializer()
            => RunInPreview(async () =>
            {
                var sample = new SampleArgs
                {
                    List =
                    {
                        "testValue",
                        123,
                        Union<string, int>.FromT0("left"),
                        Union<string, int>.FromT1(456),
                    }
                };
                var data = await sample.List.ToOutput().DataTask.ConfigureAwait(false);
                Assert.Equal(4, data.Value.Length);
                Assert.True(data.Value.IndexOf("testValue") >= 0);
                Assert.True(data.Value.IndexOf(123) >= 0);
                Assert.True(data.Value.IndexOf("left") >= 0);
                Assert.True(data.Value.IndexOf(456) >= 0);
            });

        [Fact]
        public Task InputUnionInitializer()
            => RunInPreview(async () =>
            {
                var sample = new SampleArgs{ Union = "testValue" };
                var data = await sample.Union.ToOutput().DataTask.ConfigureAwait(false);
                Assert.Equal("testValue", data.Value);

                sample = new SampleArgs{ Union = 123 };
                data = await sample.Union.ToOutput().DataTask.ConfigureAwait(false);
                Assert.Equal(123, data.Value);

                sample = new SampleArgs{ Union = Union<string, int>.FromT0("left") };
                data = await sample.Union.ToOutput().DataTask.ConfigureAwait(false);
                Assert.Equal("left", data.Value);

                sample = new SampleArgs{ Union = Union<string, int>.FromT1(456) };
                data = await sample.Union.ToOutput().DataTask.ConfigureAwait(false);
                Assert.Equal(456, data.Value);
            });

        private class SampleArgs
        {
            public readonly InputList<Union<string, int>> List = new InputList<Union<string, int>>();
            public readonly InputMap<Union<string, int>> Dict = new InputMap<Union<string, int>>();
            public InputUnion<string, int> Union = new InputUnion<string, int>();
        }
    }
}
