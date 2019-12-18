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
        private readonly Serializer _serializer = new Serializer();
        
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
            var result = await _mocks.CallAsync(request.Tok, ToDictionary(request.Args), request.Provider)
                .ConfigureAwait(false);
            return new InvokeResponse {Return = await SerializeAsync(result).ConfigureAwait(false)};
        }

        public async Task<ReadResourceResponse> ReadResourceAsync(Resource resource, ReadResourceRequest request)
        {
            var (id, state) = await _mocks.NewResourceAsync(request.Type, request.Name,
                ToDictionary(request.Properties), request.Provider, request.Id).ConfigureAwait(false);

            lock (this.Resources)
            {
                this.Resources.Add(resource);
            }

            return new ReadResourceResponse
            {
                Urn = NewUrn(request.Parent, request.Type, request.Name),
                Properties = await SerializeAsync(state).ConfigureAwait(false) 
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

            var (id, state) = await _mocks.NewResourceAsync(request.Type, request.Name, ToDictionary(request.Object),
                request.Provider, request.ImportId).ConfigureAwait(false);
            
            return new RegisterResourceResponse
            {
                Id = id ?? request.ImportId,
                Urn = NewUrn(request.Parent, request.Type, request.Name),
                Object = await SerializeAsync(state).ConfigureAwait(false) 
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
            return "urn:pulumi:" + string.Join("::", new[] { Deployment.Instance.StackName, Deployment.Instance.ProjectName, type, name });
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

        private async Task<Struct> SerializeAsync(object o)
        {
            var dict = (o as IDictionary<string, object>)?.ToImmutableDictionary()
                       ?? await _serializer.SerializeAsync("", o, true).ConfigureAwait(false) as ImmutableDictionary<string, object>
                       ?? throw new InvalidOperationException($"{o.GetType().FullName} is not a supported argument type");
            return Serializer.CreateStruct(dict);
        }
    }
}
