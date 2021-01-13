using System;
using System.Collections.Generic;
using System.Reflection;

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
        /// The collection of assemblies containing all necessary <see cref="CustomResource"/> implementations.
        /// Only comes into play during inline program execution, when a <see cref="Program"/> is provided.
        /// <para/>
        /// Useful when control is needed over what assemblies Pulumi should search for <see cref="CustomResource"/>
        /// discovery - such as when the executing assembly does not itself reference the assemblies that contain
        /// those implementations.
        /// <para/>
        /// If not provided, <see cref="AppDomain.CurrentDomain"/> and its referenced assemblies will be used
        /// for <see cref="CustomResource"/> discovery.
        /// </summary>
        public IList<Assembly>? ResourcePackageAssemblies { get; set; }

        /// <summary>
        /// Environment values scoped to the current workspace. These will be supplied to every
        /// Pulumi command.
        /// </summary>
        public IDictionary<string, string>? EnvironmentVariables { get; set; }

        /// <summary>
        /// The settings object for the current project.
        /// <para/>
        /// If provided when initializing <see cref="LocalWorkspace"/> a project settings
        /// file will be written to when the workspace is initialized via
        /// <see cref="LocalWorkspace.SaveProjectSettingsAsync(ProjectSettings, System.Threading.CancellationToken)"/>.
        /// </summary>
        public ProjectSettings? ProjectSettings { get; set; }

        /// <summary>
        /// A map of Stack names and corresponding settings objects.
        /// <para/>
        /// If provided when initializing <see cref="LocalWorkspace"/> stack settings
        /// file(s) will be written to when the workspace is initialized via
        /// <see cref="LocalWorkspace.SaveStackSettingsAsync(string, Automation.StackSettings, System.Threading.CancellationToken)"/>.
        /// </summary>
        public IDictionary<string, StackSettings>? StackSettings { get; set; }
    }
}
