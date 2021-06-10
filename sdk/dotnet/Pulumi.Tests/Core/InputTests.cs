// Copyright 2016-2019, Pulumi Corporation

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
        public Task InputMapUnionInitializer()
            => RunInPreview(async () =>
            {
                var sample = new SampleArgs
                {
                    Dict =
                    {
                        { "left", "testValue" },
                        { "right", 123 }
                    }
                };
                var data = await sample.Dict.ToOutput().DataTask.ConfigureAwait(false);
                Assert.Equal(2, data.Value.Count);
                Assert.True(data.Value.ContainsValue("testValue"));
                Assert.True(data.Value.ContainsValue(123));
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
                        123
                    }
                };
                var data = await sample.List.ToOutput().DataTask.ConfigureAwait(false);
                Assert.Equal(2, data.Value.Length);
                Assert.True(data.Value.IndexOf("testValue") >= 0);
                Assert.True(data.Value.IndexOf(123) >= 0);
            });

        private class SampleArgs
        {
            public readonly InputList<Union<string, int>> List = new InputList<Union<string, int>>();
            public readonly InputMap<Union<string, int>> Dict = new InputMap<Union<string, int>>();
        }
    }
}
