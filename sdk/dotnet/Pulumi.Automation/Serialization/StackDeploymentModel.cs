// Copyright 2016-2021, Pulumi Corporation

using Pulumi.Automation.Types;

namespace Pulumi.Automation.Serialization
{
    internal class StackDeploymentModel : Json.IJsonModel<StackDeployment>
    {
        public int Version { get; set; }

        public DeploymentV3 Deployment { get; set; } = null!;

        public StackDeployment Convert() =>
            new StackDeployment(Version, Deployment);
    }
}
