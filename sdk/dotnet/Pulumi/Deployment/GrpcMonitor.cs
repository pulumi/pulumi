// Copyright 2016-2020, Pulumi Corporation

using System;
using System.Threading.Tasks;
using Grpc.Net.Client;
using Pulumirpc;
using Grpc.Core;
using System.Collections.Generic;
using System.Collections.Concurrent;

namespace Pulumi
{
    internal class GrpcMonitor : IMonitor
    {
        private readonly ResourceMonitor.ResourceMonitorClient _client;
        // Using a static dictionary to keep track of and re-use gRPC channels
        // According to the docs (https://docs.microsoft.com/en-us/aspnet/core/grpc/performance?view=aspnetcore-6.0#reuse-grpc-channels), creating GrpcChannels is expensive so we keep track of a bunch of them here
        private static readonly ConcurrentDictionary<string, GrpcChannel> _monitorChannels = new ConcurrentDictionary<string, GrpcChannel>();
        private static readonly object _channelsLock = new object();
        public GrpcMonitor(string monitorAddress)
        {
            // Allow for insecure HTTP/2 transport (only needed for netcoreapp3.x)
            // https://docs.microsoft.com/en-us/aspnet/core/grpc/troubleshoot?view=aspnetcore-6.0#call-insecure-grpc-services-with-net-core-client
            AppContext.SetSwitch("System.Net.Http.SocketsHttpHandler.Http2UnencryptedSupport", true);
            // maxRpcMessageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
            const int maxRpcMessageSize = 400 * 1024 * 1024;
            lock (_channelsLock)
            {
                if (!_monitorChannels.ContainsKey(monitorAddress))
                {
                    // Inititialize the monitor channel once for this monitor address
                    var monitorChannel = GrpcChannel.ForAddress(new Uri($"http://{monitorAddress}"), new GrpcChannelOptions
                    {
                        MaxReceiveMessageSize = maxRpcMessageSize,
                        MaxSendMessageSize = maxRpcMessageSize,
                        Credentials = ChannelCredentials.Insecure
                    });

                    _monitorChannels.TryAdd(monitorAddress, monitorChannel);
                }
            }

            this._client = new ResourceMonitor.ResourceMonitorClient(_monitorChannels[monitorAddress]);
        }
        
        public async Task<SupportsFeatureResponse> SupportsFeatureAsync(SupportsFeatureRequest request)
            => await this._client.SupportsFeatureAsync(request);

        public async Task<InvokeResponse> InvokeAsync(InvokeRequest request)
            => await this._client.InvokeAsync(request);

        public async Task<CallResponse> CallAsync(CallRequest request)
            => await this._client.CallAsync(request);
        
        public async Task<ReadResourceResponse> ReadResourceAsync(Resource resource, ReadResourceRequest request)
            => await this._client.ReadResourceAsync(request);

        public async Task<RegisterResourceResponse> RegisterResourceAsync(Resource resource, RegisterResourceRequest request)
            => await this._client.RegisterResourceAsync(request);
        
        public async Task RegisterResourceOutputsAsync(RegisterResourceOutputsRequest request)
            => await this._client.RegisterResourceOutputsAsync(request);
    }
}
