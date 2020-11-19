namespace Pulumi.X.Automation
{
    public class InlineProgramArgs : LocalWorkspaceOptions
    {
        public string ProjectName { get; }

        public string StackName { get; }

        public InlineProgramArgs(
            string projectName,
            string stackName,
            PulumiFn program)
        {
            this.ProjectName = projectName;
            this.StackName = stackName;
            this.Program = program;
        }
    }
}
