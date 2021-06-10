// Copyright 2016-2020, Pulumi Corporation

using System.Collections.Generic;
using System.Threading.Tasks;
using Grpc.Core;
using Pulumirpc;

namespace Pulumi
{
    internal class GrpcEngine : IEngine
    {
        private readonly Engine.EngineClient _engine;

        public GrpcEngine(string engine)
        {
            // maxRpcMessageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
            var maxRpcMessageSize = 400 * 1024 * 1024;
            var grpcChannelOptions = new List<ChannelOption> { new ChannelOption(ChannelOptions.MaxReceiveMessageLength, maxRpcMessageSize)};
            this._engine = new Engine.EngineClient(new Channel(engine, ChannelCredentials.Insecure, grpcChannelOptions));
        }
        
        public async Task LogAsync(LogRequest request)
            => await this._engine.LogAsync(request);
        
        public async Task<SetRootResourceResponse> SetRootResourceAsync(SetRootResourceRequest request)
            => await this._engine.SetRootResourceAsync(request);

        public async Task<GetRootResourceResponse> GetRootResourceAsync(GetRootResourceRequest request)
            => await this._engine.GetRootResourceAsync(request);
    }
}
