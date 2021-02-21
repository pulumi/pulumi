// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;

namespace Pulumi.Automation
{
    public sealed class UpdateSummary
    {
        // pre-update information
        public UpdateKind Kind { get; }

        public DateTimeOffset StartTime { get; }

        public string Message { get; }

        public IImmutableDictionary<string, string> Environment { get; }

        public IImmutableDictionary<string, ConfigValue> Config { get; }

        // post-update information
        public UpdateState Result { get; }

        public DateTimeOffset EndTime { get; }

        public int? Version { get; }

        public string? Deployment { get; }

        public IImmutableDictionary<OperationType, int>? ResourceChanges { get; }

        internal UpdateSummary(
            UpdateKind kind,
            DateTimeOffset startTime,
            string message,
            IDictionary<string, string> environment,
            IDictionary<string, ConfigValue> config,
            UpdateState result,
            DateTimeOffset endTime,
            int? version,
            string? deployment,
            IDictionary<OperationType, int>? resourceChanges)
        {
            this.Kind = kind;
            this.StartTime = startTime;
            this.Message = message;
            this.Environment = environment.ToImmutableDictionary();
            this.Config = config.ToImmutableDictionary();
            this.Result = result;
            this.EndTime = endTime;
            this.Version = version;
            this.Deployment = deployment;
            this.ResourceChanges = resourceChanges?.ToImmutableDictionary();
        }
    }
}
