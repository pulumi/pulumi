// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi
{
    /// <summary>
    /// Options to help control the behavior of <see cref="Deployment.Call{T}"/>.
    /// </summary>
    public class CallOptions
    {
        /// <summary>
        /// An optional parent to use for default options for this call (e.g. the default provider
        /// to use).
        /// </summary>
        public Resource? Parent { get; set; }

        /// <summary>
        /// An optional provider to use for this call. If no provider is supplied, the default
        /// provider for the called function's package will be used.
        /// </summary>
        public ProviderResource? Provider { get; set; }

        /// <summary>
        /// An optional version, corresponding to the version of the provider plugin that should be
        /// used when performing this call.
        /// </summary>
        public string? Version { get; set; }

        /// <summary>
        /// An optional URL. If provided, the provider plugin with exactly this download URL will
        /// be used when performing this call. This will override the URL sourced from the host
        /// package, and should be rarely used.
        /// </summary>
        public string? PluginDownloadURL { get; set; }
    }
}
