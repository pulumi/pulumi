// Copyright 2016-2020, Pulumi Corporation

using System;
using System.Threading.Tasks;
using Grpc.Core;
using Grpc.Net.Client;
using Pulumirpc;

namespace Pulumi
{
    internal class GrpcMonitor : IMonitor
    {
        private readonly ResourceMonitor.ResourceMonitorClient _client;

        public GrpcMonitor(string monitor)
        {
            // maxRpcMessageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
            var maxRpcMessageSize = 400 * 1024 * 1024;
            var grpcChannelOptions = new GrpcChannelOptions { MaxReceiveMessageSize = maxRpcMessageSize, Credentials = ChannelCredentials.Insecure };
            this._client =
                new ResourceMonitor.ResourceMonitorClient(GrpcChannel.ForAddress(new Uri($"http://{monitor}"), grpcChannelOptions));
        }
        
        public async Task<SupportsFeatureResponse> SupportsFeatureAsync(SupportsFeatureRequest request)
            => await this._client.SupportsFeatureAsync(request);

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
