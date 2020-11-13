namespace Pulumi.X.Automation
{
    /// <summary>
    /// A Pulumi project manifest. It describes metadata applying to all sub-stacks created from the project.
    /// </summary>
    public class ProjectSettings
    {
        public string Name { get; set; }

        public ProjectRuntime Runtime { get; set; }

        public string? Main { get; set; }

        public string? Description { get; set; }

        public string? Author { get; set; }

        public string? Website { get; set; }

        public string? License { get; set; }

        public string? Config { get; set; }

        public ProjectTemplate? Template { get; set; }

        public ProjectBackend? Backend { get; set; }

        public ProjectSettings(
            string name,
            ProjectRuntime runtime)
        {
            this.Name = name;
            this.Runtime = runtime;
        }
    }
}
