// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation
{
    public class InlineProgramArgs : LocalWorkspaceOptions
    {
        public string StackName { get; }

        public InlineProgramArgs(
            string projectName,
            string stackName,
            PulumiFn program)
        {
            this.ProjectSettings = ProjectSettings.Default(projectName);
            this.StackName = stackName;
            this.Program = program;
        }
    }
}
