// Copyright 2016-2018, Pulumi Corporation

using System;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Serialization;
using Pulumirpc;

namespace Pulumi
{
    public sealed partial class Deployment
    {
        Task IDeployment.InvokeAsync(string token, ResourceArgs args, InvokeOptions? options)
            => InvokeAsync<object>(token, args, options, convertResult: false);

        Task<T> IDeployment.InvokeAsync<T>(string token, ResourceArgs args, InvokeOptions? options)
            => InvokeAsync<T>(token, args, options, convertResult: true);

        private async Task<T> InvokeAsync<T>(
            string token, ResourceArgs args, InvokeOptions? options, bool convertResult)
        {
            var label = $"Invoking function: token={token} asynchronously";
            Log.Debug(label);

            // Wait for all values to be available, and then perform the RPC.
            var argsDict = args.ToDictionary();
            var serialized = await SerializeAllPropertiesAsync($"invoke:{token}", argsDict);
            Log.Debug($"Invoke RPC prepared: token={token}" +
                (_excessiveDebugOutput ? $", obj={serialized}" : ""));

            var provider = await ProviderResource.RegisterAsync(GetProvider(token, options)).ConfigureAwait(false);

            var result = await this.Monitor.InvokeAsync(new InvokeRequest
            {
                Tok = token,
                Provider = provider ?? "",
                Version = options?.Version ?? "",
                Args = serialized,
            });

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

            if (!convertResult)
            {
                return default!;
            }

            var data = Converter.ConvertValue<T>($"{token} result", new Value { StructValue = result.Return });
            return data.Value;
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
