// Copyright 2016-2021, Pulumi Corporation
// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/core.go

using System;
using System.Collections.Generic;
using System.Text.Json.Serialization;

// TODO(vipentti): Select proper namespace
namespace Pulumi.Automation.Types
{
    // TODO(vipentti): internal -> public
    // TODO(vipentti): Split into file per class
    // TODO(vipentti): Separate JSON models & public types?

    internal class VersionedDeployment
    {
        public int? Version { get; set; }

        public DeploymentV3 Deployment { get; set; } = null!;
    }

    /// <summary>
    /// <see cref="EngineOperationType"/> is the type of an operation initiated by the engine. Its value indicates the type of operation
    /// that the engine initiated.
    /// </summary>
    internal enum EngineOperationType
    {
        /// <summary>
        /// Creating is the state of resources that are being created.
        /// </summary>
        Creating,

        /// <summary>
	    /// Updating is the state of resources that are being updated.
        /// </summary>
        Updating,

        /// <summary>
        /// Deleting is the state of resources that are being deleted.
        /// </summary>
        Deleting,

        /// <summary>
        /// Reading is the state of resources that are being read.
        /// </summary>
        Reading,
    }

    internal class CustomTimeouts
    {
        public string Create { get; set; } = null!;

        public string Update { get; set; } = null!;

        public string Delete { get; set; } = null!;
    }

    /// <summary>
    /// <see cref="ManifestV1"/> captures meta-information about this checkpoint file, such as versions of binaries, etc.
    /// </summary>
    internal class ManifestV1
    {
        /// <summary>
        /// Time of the update.
        /// </summary>
        public DateTime Time { get; set; }

        /// <summary>
        /// Magic number, used to identify integrity of the checkpoint.
        /// </summary>
        public string Magic { get; set; } = null!;

        /// <summary>
        /// Version of the Pulumi engine used to render the checkpoint.
        /// </summary>
        public string Version { get; set; } = null!;

        /// <summary>
        /// Plugins contains the binary version info of plug-ins used.
        /// </summary>
        public List<PluginInfoV1>? Plugins { get; set; } = null!;
    }

    /// <summary>
    /// <see cref="PluginInfoV1"/> captures the version and information about a plugin.
    /// </summary>
    internal class PluginInfoV1
    {
        public string Name { get; set; } = null!;
        public string Path { get; set; } = null!;
        public PluginKind Type { get; set; }
        public string Version { get; set; } = null!;
    }

    internal class SecretsProvidersV1
    {
        public string Type { get; set; } = null!;

        // TODO(vipentti): Verify type
        public object? State { get; set; } = null!;
    }

    /// <summary>
    /// <see cref="OperationV2"/> represents an operation that the engine is performing. It consists of a Resource, which is the state
    /// that the engine used to initiate the operation, and a Type, which represents the operation that the engine initiated.
    /// </summary>
    internal class OperationV2
    {
        /// <summary>
        /// Resource is the state that the engine used to initiate this operation.
        /// </summary>
        public ResourceV3 Resource { get; set; } = null!;

        /// <summary>
        /// Type represents the operation that the engine is performing.
        /// </summary>
        public EngineOperationType Type { get; set; }
    }

    /// <summary>
    /// <see cref="DeploymentV3"/> is the third version of the Deployment. It contains newer versions of the
    /// Resource and Operation API types and a placeholder for a stack's secrets configuration.
    /// </summary>
    internal class DeploymentV3
    {
        /// <summary>
        /// Manifest contains metadata about this deployment.
        /// </summary>
        public ManifestV1 Manifest { get; set; } = null!;

        /// <summary>
        /// SecretsProviders is a placeholder for secret provider configuration.
        /// </summary>
        [JsonPropertyName("secrets_providers")]
        public SecretsProvidersV1? SecretsProviders { get; set; }

        /// <summary>
        /// Resources contains all resources that are currently part of this stack after this deployment has finished.
        /// </summary>
        public List<ResourceV3>? Resources { get; set; }

        /// <summary>
        /// PendingOperations are all operations that were known by the engine to be currently executing.
        /// </summary>
        [JsonPropertyName("pending_operations")]
        public List<OperationV2>? PendingOperations { get; set; }
    }

    /// <summary>
    /// <see cref="ResourceV3"/> is the third version of the Resource API type
    /// </summary>
    internal class ResourceV3
    {
        /// <summary>
        /// URN uniquely identifying this resource.
        /// </summary>
        public string Urn { get; set; } = null!;

        /// <summary>
        /// Custom is true when it is managed by a plugin.
        /// </summary>
        public bool Custom { get; set; }

        /// <summary>
        /// Delete is true when the resource should be deleted during the next update.
        /// </summary>
        public bool? Delete { get; set; }

        /// <summary>
        /// ID is the provider-assigned resource, if any, for custom resources.
        /// </summary>
        public string? Id { get; set; } = null!;

        /// <summary>
        /// Type is the resource's full type token.
        /// </summary>
        public string Type { get; set; } = null!;

        /// <summary>
        /// Inputs are the input properties supplied to the provider.
        /// </summary>
        public Dictionary<string, object>? Inputs { get; set; } = null!;

        /// <summary>
        /// Outputs are the output properties returned by the provider after provisioning.
        /// </summary>
        public Dictionary<string, object>? Outputs { get; set; } = null!;

        /// <summary>
        /// Parent is an optional parent URN if this resource is a child of it.
        /// </summary>
        public string? Parent { get; set; } = null!;

        /// <summary>
        /// Protect is set to true when this resource is "protected" and may not be deleted.
        /// </summary>
        public bool? Protect { get; set; }

        /// <summary>
        /// External is set to true when the lifecycle of this resource is not managed by Pulumi.
        /// </summary>
        public bool? External { get; set; }

        /// <summary>
        /// Dependencies contains the dependency edges to other resources that this depends on.
        /// </summary>
        public List<string>? Dependencies { get; set; } = null!;

        /// <summary>
        /// InitErrors is the set of errors encountered in the process of initializing resource (i.e.,
        /// during create or update).
        /// </summary>
        public List<string>? InitErrors { get; set; } = null!;

        /// <summary>
        /// Provider is a reference to the provider that is associated with this resource.
        /// </summary>
        public string? Provider { get; set; } = null!;

        /// <summary>
        /// PropertyDependencies maps from an input property name to the set of resources that property depends on.
        /// </summary>
        public Dictionary<string, string?>? PropertyDependencies { get; set; } = null!;

        /// <summary>
        /// PendingReplacement is used to track delete-before-replace resources that have been deleted but not yet
        /// recreated.
        /// </summary>
        public bool? PendingReplacement { get; set; }

        /// <summary>
        /// AdditionalSecretOutputs is a list of outputs that were explicitly marked as secret when the resource was created.
        /// </summary>
        public List<string>? AdditionalSecretOutputs { get; set; }

        /// <summary>
        /// Aliases is a list of previous URNs that this resource may have had in previous deployments
        /// </summary>
        public List<string>? Aliases { get; set; }

        /// <summary>
        /// CustomTimeouts is a configuration block that can be used to control timeouts of CRUD operations
        /// </summary>
        public CustomTimeouts? CustomTimeouts { get; set; }

        /// <summary>
        /// ImportID is the import input used for imported resources.
        /// </summary>
        public string? ImportId { get; set; }
    }
}
