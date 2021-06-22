// Copyright 2016-2020, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Serialization;
using Pulumirpc;

namespace Pulumi.Testing
{
    internal class MockMonitor : IMonitor
    {
        private readonly IMocks _mocks;
        private readonly Serializer _serializer = new Serializer(excessiveDebugOutput: false);
        private readonly Dictionary<string, object> _registeredResources = new Dictionary<string, object>();

        public readonly List<Resource> Resources = new List<Resource>();

        public MockMonitor(IMocks mocks)
        {
            _mocks = mocks;
        }

        public Task<SupportsFeatureResponse> SupportsFeatureAsync(SupportsFeatureRequest request)
        {
            var hasSupport = request.Id == "secrets" || request.Id == "resourceReferences";
            return Task.FromResult(new SupportsFeatureResponse { HasSupport = hasSupport });
        }

        public async Task<InvokeResponse> InvokeAsync(InvokeRequest request)
        {
            var args = ToDictionary(request.Args);

            if (request.Tok == "pulumi:pulumi:getResource")
            {
                var urn = (string)args["urn"];
                object? registeredResource;
                lock (_registeredResources)
                {
                    if (!_registeredResources.TryGetValue(urn, out registeredResource))
                    {
                        throw new InvalidOperationException($"Unknown resource {urn}");
                    }
                }
                return new InvokeResponse { Return = await SerializeAsync(registeredResource).ConfigureAwait(false) };
            }
            
            var result = await _mocks.CallAsync(new MockCallArgs
                {
                    Token = request.Tok,
                    Args = args,
                    Provider = request.Provider,
                })
                .ConfigureAwait(false);
            return new InvokeResponse { Return = await SerializeAsync(result).ConfigureAwait(false) };
        }

        public async Task<ReadResourceResponse> ReadResourceAsync(Resource resource, ReadResourceRequest request)
        {
            var (id, state) = await _mocks.NewResourceAsync(new MockResourceArgs
            {
                Type = request.Type,
                Name = request.Name,
                Inputs = ToDictionary(request.Properties),
                Provider = request.Provider,
                Id = request.Id,
            }).ConfigureAwait(false);

            var urn = NewUrn(request.Parent, request.Type, request.Name);
            var serializedState = await SerializeToDictionary(state).ConfigureAwait(false);

            lock (_registeredResources)
            {
                var builder = ImmutableDictionary.CreateBuilder<string, object>();
                builder.Add("urn", urn);
                if (id != null)
                {
                    builder.Add("id", id);
                }
                builder.Add("state", serializedState);
                _registeredResources[urn] = builder.ToImmutable();
            }

            lock (this.Resources)
            {
                this.Resources.Add(resource);
            }

            return new ReadResourceResponse
            {
                Urn = urn,
                Properties = Serializer.CreateStruct(serializedState),
            };
        }

        public async Task<RegisterResourceResponse> RegisterResourceAsync(Resource resource, RegisterResourceRequest request)
        {
            lock (this.Resources)
            {
                this.Resources.Add(resource);
            }

            if (request.Type == Stack._rootPulumiStackTypeName)
            {
                return new RegisterResourceResponse
                {
                    Urn = NewUrn(request.Parent, request.Type, request.Name),
                    Object = new Struct(),
                };
            }

            var (id, state) = await _mocks.NewResourceAsync(new MockResourceArgs
            {
                Type = request.Type,
                Name = request.Name,
                Inputs = ToDictionary(request.Object),
                Provider = request.Provider,
                Id = request.ImportId,
            }).ConfigureAwait(false);

            var urn = NewUrn(request.Parent, request.Type, request.Name);
            var serializedState = await SerializeToDictionary(state).ConfigureAwait(false);

            lock (_registeredResources)
            {
                var builder = ImmutableDictionary.CreateBuilder<string, object>();
                builder.Add("urn", urn);
                builder.Add("id", id ?? request.ImportId);
                builder.Add("state", serializedState);
                _registeredResources[urn] = builder.ToImmutable();
            }

            return new RegisterResourceResponse
            {
                Id = id ?? request.ImportId,
                Urn = urn,
                Object = Serializer.CreateStruct(serializedState),
            };
        }

        public Task RegisterResourceOutputsAsync(RegisterResourceOutputsRequest request) => Task.CompletedTask;

        private static string NewUrn(string parent, string type, string name)
        {
            if (!string.IsNullOrEmpty(parent))
            {
                var qualifiedType = parent.Split("::")[2];
                var parentType = qualifiedType.Split("$").First();
                type = parentType + "$" + type;
            }
            return "urn:pulumi:" + string.Join("::", Deployment.Instance.StackName, Deployment.Instance.ProjectName, type, name);
        }

        private static ImmutableDictionary<string, object> ToDictionary(Struct s)
        {
            var builder = ImmutableDictionary.CreateBuilder<string, object>();
            foreach (var (key, value) in s.Fields)
            {
                var data = Deserializer.Deserialize(value);
                if (data.IsKnown && data.Value != null)
                {
                    builder.Add(key, data.Value);
                }
            }
            return builder.ToImmutable();
        }

        private async Task<ImmutableDictionary<string, object>> SerializeToDictionary(object o)
        {
            if (o is IDictionary<string, object> d)
            {
                o = d.ToImmutableDictionary();
            }
            return await _serializer.SerializeAsync("", o, true).ConfigureAwait(false) as ImmutableDictionary<string, object>
                ?? throw new InvalidOperationException($"{o.GetType().FullName} is not a supported argument type");
        }

        private async Task<Struct> SerializeAsync(object o)
        {
            var dict = await SerializeToDictionary(o).ConfigureAwait(false);
            return Serializer.CreateStruct(dict);
        }
    }
}
