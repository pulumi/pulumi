using System.Collections.Generic;
using System.Threading.Tasks;

namespace Pulumi.X.Auomation 
{
    /// <summary>
    /// Workspace is the execution context containing a single Pulumi project, a program, and multiple stacks.
    /// Workspaces are used to manage the execution environment, providing various utilities such as plugin
    /// installation, environment configuration ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
    /// </summary>
    public interface IWorkspace 
    {
        /// <summary>
        /// The working directory to run Pulumi CLI commands.
        /// </summary>
        string WorkDir { get; } 

        /// <summary>
        /// The directory override for CLI metadata if set.
        /// This customizes the location of $PULUMI_HOME where metadata is stored and plugins are installed.
        /// </summary>
        string? PulumiHome { get; }

        /// <summary>
        /// The secrets provider to use for encryption and decryption of stack secrets.
        /// See: https://www.pulumi.com/docs/intro/concepts/config/#available-encryption-providers
        /// </summary>
        string? SecretsProvider { get; }

        /// <summary>
        /// The inline program `PulumiFn` to be used for Preview/Update operations if any.
        /// If none is specified, the stack will refer to ProjectSettings for this information.
        /// </summary>
        Pulumi.X.Auomation.PlumiFn? program { get; set; }

        /// <summary>
        /// Environment values scoped to the current workspace. These will be supplied to every Pulumi command.
        /// </summary>
        IDictionary<string, string>? EnvironmentVariables { get; set;}

        /// <summary>
        /// Creates and sets a new stack with the stack name, failing if one already exists.
        /// </summary>
        /// <param name="stackName">Name of the stack to create.</param>
        /// <returns>An task that completes after the stack is created successfully.</returns>
        Task CreateStack(string stackName);
    }

    /// <summary>
    /// A Pulumi program as an inline function (in process).
    /// </summary>
    public delegate void PlumiFn();
}
