// Copyright 2016-2020, Pulumi Corporation

using System.Threading.Tasks;
using Pulumirpc;

namespace Pulumi
{
    internal interface IEngine
    {
        Task LogAsync(LogRequest request);
        
        Task<SetRootResourceResponse> SetRootResourceAsync(SetRootResourceRequest request);

        Task<GetRootResourceResponse> GetRootResourceAsync(GetRootResourceRequest request);
    }
}
