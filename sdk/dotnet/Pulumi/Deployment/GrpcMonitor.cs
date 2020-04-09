// Copyright 2016-2020, Pulumi Corporation

using System.Threading.Tasks;
using Grpc.Core;
using Pulumirpc;

namespace Pulumi
{
    internal class GrpcMonitor : IMonitor
    {
        private readonly ResourceMonitor.ResourceMonitorClient _client;

        public GrpcMonitor(string monitor)
        {
            this._client = new ResourceMonitor.ResourceMonitorClient(new Channel(monitor, ChannelCredentials.Insecure));
        }
        
        public async Task<InvokeResponse> InvokeAsync(InvokeRequest request)
            => await this._client.InvokeAsync(request);
        
        public async Task<ReadResourceResponse> ReadResourceAsync(Resource resource, ReadResourceRequest request)
            => await this._client.ReadResourceAsync(request);

        public async Task<RegisterResourceResponse> RegisterResourceAsync(Resource resource, RegisterResourceRequest request)
            => await this._client.RegisterResourceAsync(request);
        
        public async Task RegisterResourceOutputsAsync(RegisterResourceOutputsRequest request)
            => await this._client.RegisterResourceOutputsAsync(request);
    }
}
