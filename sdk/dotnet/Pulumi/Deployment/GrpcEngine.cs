// Copyright 2016-2020, Pulumi Corporation

using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Net;
using System.Net.Http;
using System.Threading.Tasks;
using Grpc.Net.Client;
using Pulumirpc;

namespace Pulumi
{
    internal class GrpcEngine : IEngine
    {
        private readonly Engine.EngineClient _engine;
        // Using a static dictionary to keep track of and re-use gRPC channels
        // According to the docs (https://docs.microsoft.com/en-us/aspnet/core/grpc/performance?view=aspnetcore-6.0#reuse-grpc-channels), creating GrpcChannels is expensive so we keep track of a bunch of them here
        private static ConcurrentDictionary<string, GrpcChannel> _engineChannels = new ConcurrentDictionary<string, GrpcChannel>();

        public GrpcEngine(string engineAddress)
        {
            // maxRpcMessageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
            var maxRpcMessageSize = 400 * 1024 * 1024;
            if (!_engineChannels.ContainsKey(engineAddress))
            {
                // Allow for insecure HTTP/2 transport (only needed for netcoreapp3.x)
                // https://docs.microsoft.com/en-us/aspnet/core/grpc/troubleshoot?view=aspnetcore-6.0#call-insecure-grpc-services-with-net-core-client
                AppContext.SetSwitch("System.Net.Http.SocketsHttpHandler.Http2UnencryptedSupport", true);
                // Inititialize the engine channel once for this address
                _engineChannels[engineAddress] = GrpcChannel.ForAddress(new Uri($"http://{engineAddress}"), new GrpcChannelOptions
                {
                    MaxReceiveMessageSize = maxRpcMessageSize,
                    MaxSendMessageSize = maxRpcMessageSize,
                    Credentials = Grpc.Core.ChannelCredentials.Insecure,
                });
            }

            this._engine = new Engine.EngineClient(_engineChannels[engineAddress]);
        }
        
        public async Task LogAsync(LogRequest request)
            => await this._engine.LogAsync(request);

        public async Task<SetRootResourceResponse> SetRootResourceAsync(SetRootResourceRequest request)
            => await this._engine.SetRootResourceAsync(request);

        public async Task<GetRootResourceResponse> GetRootResourceAsync(GetRootResourceRequest request)
            => await this._engine.GetRootResourceAsync(request);
    }
}
