// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;

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

        public Action<string>? OnOutput { get; set; }

        public Action<string>? OnStdErr { get; set; }
    }
}
