// Copyright 2016-2021, Pulumi Corporation

using Pulumi.Automation.Types;

namespace Pulumi.Automation
{
    /// <summary>
    /// Represents the state of a stack deployment as used by
    /// ExportStackAsync and ImportStackAsync.
    /// <para/>
    /// NOTE: instances may contain sensitive data (secrets).
    /// <para/>
    /// </summary>
    public sealed class StackDeployment
    {
        /// <summary>
        /// Version indicates the schema of the encoded deployment.
        /// </summary>
        public int Version { get; }

        /// <summary>
        /// The deployment
        /// </summary>
        // TODO(vipentti): internal -> public
        internal DeploymentV3 Deployment { get; }

        internal StackDeployment(int version, DeploymentV3 deployment)
        {
            this.Version = version;
            this.Deployment = deployment;
        }
    }
}
