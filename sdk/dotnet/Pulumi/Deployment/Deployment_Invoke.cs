// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Rpc;
using Pulumirpc;

namespace Pulumi
{
    public class InvokeOptions
    {
        /// <summary>
        /// An optional parent to use for default options for this invoke (e.g. the default provider
        /// to use).
        /// </summary>
        public Resource? Parent { get; set; }

        /// <summary>
        /// An optional provider to use for this invocation. If no provider is supplied, the default
        /// provider for the invoked function's package will be used.
        /// </summary>
        public ProviderResource? Provider { get; set; }
        /// <summary>
        /// An optional version, corresponding to the version of the provider plugin that should be
        /// used when performing this invoke.
        /// </summary>
        public string? Version { get; set; }
    }

    public sealed partial class Deployment
    {
        public async Task<T> InvokeAsync<T>(
            string token, ResourceArgs args,
            Func<ImmutableDictionary<string, object>, T> convert, InvokeOptions? options)
        {
            var label = $"Invoking function: tok={token} asynchronously";
            Log.Debug(label);

            // Wait for all values to be available, and then perform the RPC.
            var argsDict = args.ToDictionary();
            var serialized = await SerializeAllPropertiesAsync($"invoke:{token}", argsDict);
            Log.Debug($"Invoke RPC prepared: token={token}" +
                (_excessiveDebugOutput ? $", obj={serialized}" : ""));

            var monitor = this.Monitor;
            var provider = await ProviderResource.RegisterAsync(GetProvider(token, options));

            var result = await monitor.InvokeAsync(new InvokeRequest
            {
                Tok = token,
                Provider = provider,
                Version = options?.Version ?? "",
                Args = serialized,
            });

            // Finally propagate any other properties that were given to us as outputs.
            return DeserializeResponse<T>(token, convert, result);
        }

        private T DeserializeResponse<T>(
            string token, Func<ImmutableDictionary<string, object>, T> convert, InvokeResponse result)
        {
            if (result.Failures.Count > 0)
            {
                var reasons = "";
                foreach (var reason in result.Failures)
                {
                    if (reasons != "")
                    {
                        reasons += "; ";
                    }

                    reasons += $"{reason.Reason} ({reason.Property})";
                }

                throw new InvokeException($"Invoke of '{token}' failed: {reasons}");
            }

            var (deserialized, _) = Deserializers.GenericStructDeserializer(new Value { StructValue = result.Return });
            return convert(deserialized);
        }

        private static ProviderResource? GetProvider(string token, InvokeOptions? options)
                => options?.Provider ?? options?.Parent?.GetProvider(token);

        private class InvokeException : Exception
        {
            public InvokeException(string error)
                : base(error)
            {
            }
        }
    }
}
