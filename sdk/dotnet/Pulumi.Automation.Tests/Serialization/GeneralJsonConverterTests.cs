// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using Pulumi.Automation.Serialization;
using Xunit;

namespace Pulumi.Automation.Tests.Serialization
{
    public class GeneralJsonConverterTests
    {
        private static readonly LocalSerializer _serializer = new LocalSerializer();

        [Fact]
        public void CanDeserializeConfigValue()
        {
            var json = @"
{
    ""aws:region"": {
        ""value"": ""us-east-1"",
        ""secret"": false
    },
    ""project:name"": {
        ""value"": ""test"",
        ""secret"": true
    }
}
";

            var config = _serializer.DeserializeJson<Dictionary<string, ConfigValue>>(json);
            Assert.NotNull(config);
            Assert.True(config.TryGetValue("aws:region", out var regionValue));
            Assert.Equal("us-east-1", regionValue!.Value);
            Assert.False(regionValue.IsSecret);
            Assert.True(config.TryGetValue("project:name", out var secretValue));
            Assert.Equal("test", secretValue!.Value);
            Assert.True(secretValue.IsSecret);
        }

        [Fact]
        public void CanDeserializePluginInfo()
        {
            var json = @"
{
    ""name"": ""aws"",
    ""kind"": ""resource"",
    ""version"": ""3.19.2"",
    ""size"": 258460028,
    ""installTime"": ""2020-12-09T19:24:23.214Z"",
    ""lastUsedTime"": ""2020-12-09T19:24:26.059Z""
}
";
            var installTime = new DateTime(2020, 12, 9, 19, 24, 23, 214);
            var lastUsedTime = new DateTime(2020, 12, 9, 19, 24, 26, 059);

            var info = _serializer.DeserializeJson<PluginInfo>(json);
            Assert.NotNull(info);
            Assert.Equal("aws", info.Name);
            Assert.Equal(PluginKind.Resource, info.Kind);
            Assert.Equal(258460028, info.Size);
            Assert.Equal(new DateTimeOffset(installTime, TimeSpan.Zero), info.InstallTime);
            Assert.Equal(new DateTimeOffset(lastUsedTime, TimeSpan.Zero), info.LastUsedTime);
        }

        [Fact]
        public void CanDeserializeUpdateSummary()
        {
            var json = @"
[
  {
    ""kind"": ""destroy"",
    ""startTime"": ""2021-01-07T17:08:49.000Z"",
    ""message"": """",
    ""environment"": {
        ""exec.kind"": ""cli""
    },
    ""config"": {
        ""aws:region"": {
            ""value"": ""us-east-1"",
            ""secret"": false
        },
        ""quickstart:test"": {
            ""value"": ""okok"",
            ""secret"": true
        }
    },
    ""result"": ""in-progress"",
    ""endTime"": ""2021-01-07T17:09:14.000Z"",
    ""resourceChanges"": {
        ""delete"": 3,
        ""discard"": 1
    }
  },
  {
    ""kind"": ""update"",
    ""startTime"": ""2021-01-07T17:02:10.000Z"",
    ""message"": """",
    ""environment"": {
        ""exec.kind"": ""cli""
    },
    ""config"": {
        ""aws:region"": {
            ""value"": ""us-east-1"",
            ""secret"": false
        },
        ""quickstart:test"": {
            ""value"": ""okok"",
            ""secret"": true
        }
    },
    ""result"": ""succeeded"",
    ""endTime"": ""2021-01-07T17:02:24.000Z"",
    ""resourceChanges"": {
      ""create"": 3
    }
  }
]
";

            var history = _serializer.DeserializeJson<List<UpdateSummary>>(json);
            Assert.NotNull(history);
            Assert.Equal(2, history.Count);

            var destroy = history[0];
            Assert.Equal(UpdateKind.Destroy, destroy.Kind);
            Assert.Equal(UpdateState.InProgress, destroy.Result);
            Assert.NotNull(destroy.ResourceChanges);
            Assert.Equal(2, destroy.ResourceChanges!.Count);
            Assert.True(destroy.ResourceChanges.TryGetValue(OperationType.Delete, out var deletedCount));
            Assert.Equal(3, deletedCount);
            Assert.True(destroy.ResourceChanges.TryGetValue(OperationType.ReadDiscard, out var discardCount));
            Assert.Equal(1, discardCount);


            var update = history[1];
            Assert.Equal(UpdateKind.Update, update.Kind);
            Assert.Equal(UpdateState.Succeeded, update.Result);
            Assert.NotNull(update.ResourceChanges);
            Assert.Equal(1, update.ResourceChanges!.Count);
            Assert.True(update.ResourceChanges.TryGetValue(OperationType.Create, out var createdCount));
            Assert.Equal(3, createdCount);
        }
    }
}
