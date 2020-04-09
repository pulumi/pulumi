// Copyright 2016-2020, Pulumi Corporation

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
            this._engine = new Engine.EngineClient(new Channel(engine, ChannelCredentials.Insecure));
        }
        
        public async Task LogAsync(LogRequest request)
            => await this._engine.LogAsync(request);
        
        public async Task<SetRootResourceResponse> SetRootResourceAsync(SetRootResourceRequest request)
            => await this._engine.SetRootResourceAsync(request);

        public async Task<GetRootResourceResponse> GetRootResourceAsync(GetRootResourceRequest request)
            => await this._engine.GetRootResourceAsync(request);
    }
}
