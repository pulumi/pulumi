// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation
{
    /// <summary>
    /// A placeholder config value for a project template.
    /// </summary>
    public class ProjectTemplateConfigValue
    {
        public string? Description { get; set; }

        public string? Default { get; set; }

        public bool? Secret { get; set; }
    }
}
