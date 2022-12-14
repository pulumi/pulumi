// Copyright 2016-2022, Pulumi Corporation

namespace Pulumi.Automation
{
    /// <summary>
    /// Authentication options for the repository that can be specified for a private Git repo.
    /// There are three different authentication paths:
    ///
    /// <list type="bullet">
    /// <item><description>Personal accesstoken</description></item>
    /// <item><description>SSH private key (and its optional password)</description></item>
    /// <item><description>Basic auth username and password</description></item>
    /// </list>
    ///
    /// Only one authentication path is valid.
    /// </summary>
    public class RemoteGitAuthArgs
    {
        /// <summary>
        /// The absolute path to a private key for access to the git repo.
        /// </summary>
        public string? SshPrivateKeyPath { get; set; }

        /// <summary>
        /// The (contents) private key for access to the git repo.
        /// </summary>
        public string? SshPrivateKey { get; set; }

        /// <summary>
        /// The password that pairs with a username or as part of an SSH Private Key.
        /// </summary>
        public string? Password { get; set; }

        /// <summary>
        /// PersonalAccessToken is a Git personal access token in replacement of your password.
        /// </summary>
        public string? PersonalAccessToken { get; set; }

        /// <summary>
        /// Username is the username to use when authenticating to a git repository.
        /// </summary>
        public string? Username { get; set; }
    }
}
