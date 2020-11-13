using System.Collections.Generic;

namespace Pulumi.X.Automation
{
    /// <summary>
    /// Extensibility options to configure a LocalWorkspace; e.g: settings to seed
    /// and environment variables to pass through to every command.
    /// </summary>
    public class LocalWorkspaceOptions
    {
        /// <summary>
        /// The directory to run Pulumi commands and read settings (Pulumi.yaml and Pulumi.{stack}.yaml).
        /// </summary>
        public string? WorkDir { get; set; }

        /// <summary>
        /// The directory to override for CLI metadata.
        /// </summary>
        public string? PulumiHome { get; set; }

        /// <summary>
        /// The secrets provider to user for encryption and decryption of stack secrets.
        /// <para/>
        /// See: https://www.pulumi.com/docs/intro/concepts/config/#available-encryption-providers
        /// </summary>
        public string? SecretsProvider { get; set; }

        /// <summary>
        /// The inline program <see cref="PulumiFn"/> to be used for Preview/Update operations if any.
        /// <para/>
        /// If none is specified, the stack will refer to <see cref="Automation.ProjectSettings"/> for this information.
        /// </summary>
        public PulumiFn? Program { get; set; }

        /// <summary>
        /// Environment values scoped to the current workspace. These will be supplied to every
        /// Pulumi command.
        /// </summary>
        public IDictionary<string, string>? EnvironmentVariables { get; set; }

        /// <summary>
        /// The settings object for the current project.
        /// </summary>
        public ProjectSettings? ProjectSettings { get; set; }

        /// <summary>
        /// A map of Stack names and corresponding settings objects.
        /// </summary>
        public IDictionary<string, StackSettings>? StackSettings { get; set; }
    }
}
