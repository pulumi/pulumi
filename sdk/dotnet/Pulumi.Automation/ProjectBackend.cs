// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation
{
    /// <summary>
    /// Configuration for the project's Pulumi state storage backend.
    /// </summary>
    public class ProjectBackend
    {
        public string? Url { get; set; }
    }
}
