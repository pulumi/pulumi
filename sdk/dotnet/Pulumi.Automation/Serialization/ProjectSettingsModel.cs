// Copyright 2016-2021, Pulumi Corporation

using Pulumi.Automation.Serialization.Json;

namespace Pulumi.Automation.Serialization
{
    // necessary for constructor deserialization
    internal class ProjectSettingsModel : IJsonModel<ProjectSettings>
    {
        public string? Name { get; set; }

        public ProjectRuntime? Runtime { get; set; }

        public string? Main { get; set; }

        public string? Description { get; set; }

        public string? Author { get; set; }

        public string? Website { get; set; }

        public string? License { get; set; }

        public string? Config { get; set; }

        public ProjectTemplate? Template { get; set; }

        public ProjectBackend? Backend { get; set; }

        public ProjectSettings Convert()
            => new ProjectSettings(this.Name!, this.Runtime ?? new ProjectRuntime(ProjectRuntimeName.NodeJS))
            {
                Main = this.Main,
                Description = this.Description,
                Author = this.Author,
                Website = this.Website,
                License = this.License,
                Config = this.Config,
                Template = this.Template,
                Backend = this.Backend,
            };
    }
}
