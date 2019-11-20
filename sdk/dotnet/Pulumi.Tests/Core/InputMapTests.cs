// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading;
using System.Threading.Tasks;
using Pulumi.Serialization;
using Xunit;

namespace Pulumi.Tests.Core
{
    public partial class InputMapTests : PulumiTest
    {
        [Fact]
        public Task MergeInputMaps()
            => RunInPreview(async () =>
            {
                var map1 = new InputMap<string>();
                map1.Add("K1", "V1");
                map1.Add("K2", Output.Create("V2"));
                map1.Add("K3", Output.Create("V3_wrong"));

                var map2 = new InputMap<string>();
                map2.Add("K3", Output.Create("V3"));
                map2.Add("K4", "V4");

                var result = InputMap<string>.Merge(map1, map2);

                // Check the merged map
                var data = await result.ToOutput().DataTask.ConfigureAwait(false);
                Assert.True(data.IsKnown);
                Assert.Equal(4, data.Value.Count);
                for (int i = 1; i <=4; i++)
                    Assert.True(data.Value.Contains($"K{i}", $"V{i}"));

                // Check that the input maps haven't changed
                var map1Data = await map1.ToOutput().DataTask.ConfigureAwait(false);
                Assert.Equal(3, map1Data.Value.Count);
                Assert.True(map1Data.Value.ContainsValue("V3_wrong"));

                var map2Data = await map2.ToOutput().DataTask.ConfigureAwait(false);
                Assert.Equal(2, map2Data.Value.Count);
                Assert.True(map2Data.Value.ContainsValue("V3"));
            });
    }
}
