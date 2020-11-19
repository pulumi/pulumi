using System;

namespace Pulumi.X.Automation
{
    public class PluginInfo
    {
        public string Name { get; }

        public string Path { get; }

        public PluginKind Kind { get; }

        public string? Version { get; }

        public long Size { get; } // TODO: or double? will know once get to implementation

        public DateTimeOffset InstallTime { get; }

        public DateTimeOffset LastUsedTime { get; }

        public string ServerUrl { get; }

        public PluginInfo(
            string name,
            string path,
            PluginKind kind,
            string? version,
            long size,
            DateTimeOffset installTime,
            DateTimeOffset lastUsedTime,
            string serverUrl)
        {
            this.Name = name;
            this.Path = path;
            this.Kind = kind;
            this.Version = version;
            this.Size = size;
            this.InstallTime = installTime;
            this.LastUsedTime = lastUsedTime;
            this.ServerUrl = serverUrl;
        }
    }
}
