// Copyright 2016-2020, Pulumi Corporation

using System.Threading.Tasks;
using Pulumirpc;

namespace Pulumi
{
    internal interface IMonitor
    {
		Task<SupportsFeatureResponse> SupportsFeatureAsync(SupportsFeatureRequest request);

        Task<InvokeResponse> InvokeAsync(InvokeRequest request);
        
        Task<ReadResourceResponse> ReadResourceAsync(Resource resource, ReadResourceRequest request);
        
        Task<RegisterResourceResponse> RegisterResourceAsync(Resource resource, RegisterResourceRequest request);
        
        Task RegisterResourceOutputsAsync(RegisterResourceOutputsRequest request);
    }
}
