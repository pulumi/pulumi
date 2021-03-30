// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using Pulumi.Automation.Events;

namespace Pulumi.Automation
{
    /// <summary>
    /// Common options controlling the behavior of update actions taken
    /// against an instance of <see cref="WorkspaceStack"/>.
    /// </summary>
    public class UpdateOptions
    {
        public int? Parallel { get; set; }

        public string? Message { get; set; }

        public List<string>? Target { get; set; }

        /// <summary>
        /// Optional callback which is invoked whenever StandardOutput is written into
        /// </summary>
        public Action<string>? OnStandardOutput { get; set; }

        /// <summary>
        /// Optional callback which is invoked whenever StandardError is written into
        /// </summary>
        public Action<string>? OnStandardError { get; set; }

        /// <summary>
        /// Optional callback which is invoked with the engine events
        /// </summary>
        public Action<EngineEvent>? OnEvent { get; set; }
    }
}
