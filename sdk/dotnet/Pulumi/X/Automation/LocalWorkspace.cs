using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.IO;
using System.Linq;
using System.Threading.Tasks;

namespace Pulumi.X.Automation
{
    /// <summary>
    /// LocalWorkspace is a default implementation of the Workspace interface.
    /// <para/>
    /// A Workspace is the execution context containing a single Pulumi project, a program,
    /// and multiple stacks.Workspaces are used to manage the execution environment,
    /// providing various utilities such as plugin installation, environment configuration
    /// ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
    /// <para/>
    /// LocalWorkspace relies on Pulumi.yaml and Pulumi.{stack}.yaml as the intermediate format
    /// for Project and Stack settings.Modifying ProjectSettings will
    /// alter the Workspace Pulumi.yaml file, and setting config on a Stack will modify the Pulumi.{stack}.yaml file.
    /// This is identical to the behavior of Pulumi CLI driven workspaces.
    /// </summary>
    public class LocalWorkspace : IWorkspace
    {
        private readonly Task _readyTask;

        public string WorkDir { get; }

        public string? PulumiHome { get; }

        public string? SecretsProvider { get; }

        public PulumiFn? Program { get; set; }
        
        public IDictionary<string, string>? EnvironmentVariables { get; set; }

        private LocalWorkspace(LocalWorkspaceOptions? options = null)
        {
            string? dir = null;
            var readyTasks = new List<Task>();

            if (options != null)
            {
                if (!string.IsNullOrWhiteSpace(options.WorkDir))
                    dir = options.WorkDir;

                this.PulumiHome = options.PulumiHome;
                this.Program = options.Program;
                this.SecretsProvider = options.SecretsProvider;

                if (options.EnvironmentVariables != null)
                    this.EnvironmentVariables = new Dictionary<string, string>(options.EnvironmentVariables);

                if (options.ProjectSettings != null)
                    readyTasks.Add(this.SaveProjectSettingsAsync(options.ProjectSettings));

                if (options.StackSettings != null && options.StackSettings.Any())
                {
                    foreach (var pair in options.StackSettings)
                        readyTasks.Add(this.SaveStackSettingsAsync(pair.Key, pair.Value));
                }
            }

            if (string.IsNullOrWhiteSpace(dir))
            {
                dir = Path.Combine(Path.GetTempPath(), "automation-"); // TODO: should this not be randomized in some way?
                Directory.CreateDirectory(dir);
            }

            this.WorkDir = dir;
            this._readyTask = Task.WhenAll(readyTasks);
        }

        public Task<ProjectSettings?> GetProjectSettingsAsync()
        {
            throw new NotImplementedException();
        }

        public Task SaveProjectSettingsAsync(ProjectSettings settings)
        {
            throw new NotImplementedException();
        }

        public Task<StackSettings?> GetStackSettingsAsync(string stackName)
        {
            throw new NotImplementedException();
        }

        public Task SaveStackSettingsAsync(string stackName, StackSettings settings)
        {
            throw new NotImplementedException();
        }

        public Task<ImmutableList<string>> SerializeArgsForOpAsync(string stackName)
        {
            throw new NotImplementedException();
        }

        public Task PostCommandCallbackAsync(string stackName)
        {
            throw new NotImplementedException();
        }

        public Task<ConfigValue> GetConfigAsync(string stackName, string key)
        {
            throw new NotImplementedException();
        }

        public Task<ImmutableDictionary<string, ConfigValue>> GetAllConfigAsync(string stackName)
        {
            throw new NotImplementedException();
        }

        public Task SetConfigAsync(string stackName, string key, ConfigValue value)
        {
            throw new NotImplementedException();
        }

        public Task SetAllConfigAsync(string stackName, IDictionary<string, ConfigValue> configMap)
        {
            throw new NotImplementedException();
        }

        public Task RemoveConfigAsync(string stackName, string key)
        {
            throw new NotImplementedException();
        }

        public Task RemoveAllConfigAsync(string stackName, IEnumerable<string> keys)
        {
            throw new NotImplementedException();
        }

        public Task<ImmutableDictionary<string, ConfigValue>> RefreshConfigAsync(string stackName)
        {
            throw new NotImplementedException();
        }

        public Task<WhoAmIResult> WhoAmIAsync()
        {
            throw new NotImplementedException();
        }

        public Task<StackInfo?> GetStackAsync()
        {
            throw new NotImplementedException();
        }

        public Task CreateStackAsync(string stackName)
        {
            throw new NotImplementedException();
        }

        public Task SelectStackAsync(string stackName)
        {
            throw new NotImplementedException();
        }

        public Task RemoveStackAsync(string stackName)
        {
            throw new NotImplementedException();
        }

        public Task<ImmutableList<StackInfo>> ListStacksAsync()
        {
            throw new NotImplementedException();
        }

        public Task InstallPluginAsync(string name, string version, string? kind = null)
        {
            throw new NotImplementedException();
        }

        public Task RemovePluginAsync(string? name = null, string? versionRange = null, string? kind = null)
        {
            throw new NotImplementedException();
        }

        public Task<ImmutableList<PluginInfo>> ListPluginsAsync()
        {
            throw new NotImplementedException();
        }
    }
}
