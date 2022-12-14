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

        public List<string>? PolicyPacks { get; set; }

        public List<string>? PolicyPackConfigs { get; set; }

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

        /// <summary>
        /// Colorize output. Choices are: always, never, raw, auto (default "auto")
        /// </summary>
        public string? Color { get; set; }

        /// <summary>
        /// Flow log settings to child processes (like plugins)
        /// </summary>
        public bool? LogFlow { get; set; }

        /// <summary>
        /// Enable verbose logging (e.g., v=3); anything >3 is very verbose
        /// </summary>
        public int? LogVerbosity { get; set; }

        /// <summary>
        /// Log to stderr instead of to files
        /// </summary>
        public bool? LogToStdErr { get; set; }

        /// <summary>
        /// Emit tracing to the specified endpoint. Use the file: scheme to write tracing data to a local file
        /// </summary>
        public string? Tracing { get; set; }

        /// <summary>
        /// Print detailed debugging output during resource operations
        /// </summary>
        public bool? Debug { get; set; }

        /// <summary>
        /// Format standard output as JSON not text.
        /// </summary>
        public bool? Json { get; set; }
    }
}
