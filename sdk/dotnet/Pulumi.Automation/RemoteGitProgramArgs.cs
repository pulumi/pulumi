// Copyright 2016-2022, Pulumi Corporation

namespace Pulumi.Automation
{
    /// <summary>
    /// Description of a stack backed by a remote Pulumi program in a Git repository.
    /// </summary>
    public class RemoteGitProgramArgs : RemoteWorkspaceOptions
    {
        /// <summary>
        /// The name of the associated Stack.
        /// </summary>
        public string StackName { get; }

        /// <summary>
        /// The URL of the repository.
        /// </summary>
        public string Url { get; }

        public RemoteGitProgramArgs(
            string stackName,
            string url)
        {
            StackName = stackName;
            Url = url;
        }

        /// <summary>
        /// Optional path relative to the repo root specifying location of the Pulumi program.
        /// </summary>
        public string? ProjectPath { get; set; }

        /// <summary>
        /// Optional branch to checkout.
        /// </summary>
        public string? Branch { get; set; }

        /// <summary>
        /// Optional commit to checkout.
        /// </summary>
        public string? CommitHash { get; set; }

        /// <summary>
        /// Authentication options for the repository.
        /// </summary>
        public RemoteGitAuthArgs? Auth { get; set; }
    }
}
