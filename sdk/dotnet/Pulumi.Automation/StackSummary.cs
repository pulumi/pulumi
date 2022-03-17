// Copyright 2016-2021, Pulumi Corporation

using System;

namespace Pulumi.Automation
{
    public class StackSummary
    {
        public string Name { get; }

        public bool IsCurrent { get; }

        public DateTimeOffset? LastUpdate { get; }

        public bool IsUpdateInProgress { get; }

        public int? ResourceCount { get; }

        public string? Url { get; }

        internal StackSummary(
            string name,
            bool isCurrent,
            DateTimeOffset? lastUpdate,
            bool isUpdateInProgress,
            int? resourceCount,
            string? url)
        {
            this.Name = name;
            this.IsCurrent = isCurrent;
            this.LastUpdate = lastUpdate;
            this.IsUpdateInProgress = isUpdateInProgress;
            this.ResourceCount = resourceCount;
            this.Url = url;
        }
    }
}
