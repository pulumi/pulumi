// Copyright 2016-2022, Pulumi Corporation

using System.Collections.Generic;

namespace Pulumi.Automation
{
    /// <summary>
    /// Extensibility options to configure a RemoteWorkspace.
    /// </summary>
    public class RemoteWorkspaceOptions
    {
        private IDictionary<string, EnvironmentVariableValue>? _environmentVariables;
        private IList<string>? _preRunCommands;

        /// <summary>
        /// Environment values scoped to the remote workspace. These will be passed to remote operations.
        /// </summary>
        public IDictionary<string, EnvironmentVariableValue> EnvironmentVariables
        {
            get => _environmentVariables ??= new Dictionary<string, EnvironmentVariableValue>();
            set => _environmentVariables = value;
        }

        /// <summary>
        /// An optional list of arbitrary commands to run before a remote Pulumi operation is invoked.
        /// </summary>
        public IList<string> PreRunCommands
        {
            get => _preRunCommands ??= new List<string>();
            set => _preRunCommands = value;
        }
    }
}
