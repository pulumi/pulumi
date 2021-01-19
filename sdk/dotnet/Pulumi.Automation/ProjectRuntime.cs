namespace Pulumi.Automation
{
    /// <summary>
    /// A description of the Project's program runtime and associated metadata.
    /// </summary>
    public class ProjectRuntime
    {
        public ProjectRuntimeName Name { get; set; }

        public ProjectRuntimeOptions? Options { get; set; }

        public ProjectRuntime(ProjectRuntimeName name)
        {
            this.Name = name;
        }
    }
}
