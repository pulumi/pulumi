// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Serialization;
using Pulumirpc;

namespace Pulumi
{
    public sealed partial class Deployment
    {
        void IDeployment.Call(string token, CallArgs args, Resource? self, CallOptions? options)
            => Call<object>(token, args, self, options, convertResult: false);

        Output<T> IDeployment.Call<T>(string token, CallArgs args, Resource? self, CallOptions? options)
            => Call<T>(token, args, self, options, convertResult: true);

        private Output<T> Call<T>(string token, CallArgs args, Resource? self, CallOptions? options, bool convertResult)
            => new Output<T>(CallAsync<T>(token, args, self, options, convertResult));

        private async Task<OutputData<T>> CallAsync<T>(
            string token, CallArgs args, Resource? self, CallOptions? options, bool convertResult)
        {
            var (result, deps) = await CallRawAsync(token, args, self, options).ConfigureAwait(false);
            if (convertResult)
            {
                var converted = Converter.ConvertValue<T>(err => Log.Warn(err, self), $"{token} result", new Value { StructValue = result });
                return new OutputData<T>(deps, converted.Value, converted.IsKnown, converted.IsSecret);
            }

            return new OutputData<T>(ImmutableHashSet<Resource>.Empty, default!, isKnown: true, isSecret: false);
        }

        private async Task<(Struct Return, ImmutableHashSet<Resource> Dependencies)> CallRawAsync(
            string token, CallArgs args, Resource? self, CallOptions? options)
        {
            var label = $"Calling function: token={token} asynchronously";
            Log.Debug(label);

            // Be resilient to misbehaving callers.
            // ReSharper disable once ConstantNullCoalescingCondition
            args ??= CallArgs.Empty;

            // Wait for all values to be available, and then perform the RPC.
            var argsDict = await args.ToDictionaryAsync().ConfigureAwait(false);

            // If we have a self arg, include it in the args.
            if (self != null)
            {
                argsDict = argsDict.SetItem("__self__", self);
            }

            var (serialized, argDependencies) = await SerializeFilteredPropertiesAsync(
                    $"call:{token}",
                    argsDict, _ => true,
                    keepResources: true,
                    keepOutputValues: await MonitorSupportsOutputValues().ConfigureAwait(false)).ConfigureAwait(false);
            Log.Debug($"Call RPC prepared: token={token}" +
                (_excessiveDebugOutput ? $", obj={serialized}" : ""));

            // Determine the provider and version to use.
            ProviderResource? provider;
            string? version;
            if (self != null)
            {
                provider = self._provider;
                version = self._version;
            }
            else
            {
                provider = GetProvider(token, options);
                version = options?.Version;
            }
            var providerReference = await ProviderResource.RegisterAsync(provider).ConfigureAwait(false);

            // Create the request.
            var request = new CallRequest
            {
                Tok = token,
                Provider = providerReference ?? "",
                Version = version ?? "",
                Args = serialized,
            };

            // Add arg dependencies to the request.
            foreach (var (argName, directDependencies) in argDependencies)
            {
                var urns = await GetAllTransitivelyReferencedResourceUrnsAsync(directDependencies).ConfigureAwait(false);
                var deps = new CallRequest.Types.ArgumentDependencies();
                deps.Urns.AddRange(urns);
                request.ArgDependencies.Add(argName, deps);
            }

            // Kick off the call.
            var result = await Monitor.CallAsync(request).ConfigureAwait(false);

            // Handle failures.
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

                throw new CallException($"Call of '{token}' failed: {reasons}");
            }

            // Unmarshal return dependencies.
            var dependencies = ImmutableHashSet.CreateBuilder<Resource>();
            foreach (var (_, returnDependencies) in result.ReturnDependencies)
            {
                foreach (var urn in returnDependencies.Urns)
                {
                    dependencies.Add(new DependencyResource(urn));
                }
            }

            return (result.Return, dependencies.ToImmutable());
        }

        private static ProviderResource? GetProvider(string token, CallOptions? options)
            => options?.Provider ?? options?.Parent?.GetProvider(token);

        private sealed class CallException : Exception
        {
            public CallException(string error)
                : base(error)
            {
            }
        }
    }
}
