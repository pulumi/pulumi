// Copyright 2016-2021, Pulumi Corporation

using System;
using Pulumi.Automation.Serialization.Json;

namespace Pulumi.Automation.Serialization
{
    // necessary for constructor deserialization
    internal class PluginInfoModel : IJsonModel<PluginInfo>
    {
        public string Name { get; set; } = null!;

        public string? Path { get; set; }

        public string Kind { get; set; } = null!;

        public string? Version { get; set; }

        public long Size { get; set; }

        public DateTimeOffset InstallTime { get; set; }

        public DateTimeOffset LastUsedTime { get; set; }

        public string? ServerUrl { get; set; }

        private PluginKind GetKind()
            => string.Equals(this.Kind, "analyzer", StringComparison.OrdinalIgnoreCase) ? PluginKind.Analyzer
            : string.Equals(this.Kind, "language", StringComparison.OrdinalIgnoreCase) ? PluginKind.Language
            : string.Equals(this.Kind, "resource", StringComparison.OrdinalIgnoreCase) ? PluginKind.Resource
            : throw new InvalidOperationException($"Invalid plugin kind: {this.Kind}");

        public PluginInfo Convert()
            => new PluginInfo(
                this.Name,
                this.Path,
                this.GetKind(),
                this.Version,
                this.Size,
                this.InstallTime,
                this.LastUsedTime,
                this.ServerUrl);
    }
}
