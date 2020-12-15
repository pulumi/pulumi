namespace Pulumi.X.Automation.Serialization
{
    // necessary because this version of System.Text.Json
    // can't deserialize a type that doesn't have a parameterless constructor
    internal class ProjectSettingsModel
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
        {
            return new ProjectSettings(this.Name!, this.Runtime ?? new ProjectRuntime(ProjectRuntimeName.NodeJS))
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
}
