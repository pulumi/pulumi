// Copyright 2016-2020, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Threading.Tasks;
using Pulumirpc;

namespace Pulumi.Testing
{
    internal class MockEngine : IEngine
    {
        private string? _rootResourceUrn;
        private readonly object _rootResourceUrnLock = new object();
        
        public readonly List<string> Errors = new List<string>();

        public Task LogAsync(LogRequest request)
        {
            if (request.Severity == LogSeverity.Error)
            {
                lock (this.Errors)
                {
                    this.Errors.Add(request.Message);
                }
            }
            
            return Task.CompletedTask;
        }

        public Task<SetRootResourceResponse> SetRootResourceAsync(SetRootResourceRequest request)
        {
            lock (_rootResourceUrnLock)
            {
                if (_rootResourceUrn != null && _rootResourceUrn != request.Urn)
                    throw new InvalidOperationException(
                        $"An invalid attempt to set the root resource to {request.Urn} while it's already set to {_rootResourceUrn}");
                
                _rootResourceUrn = request.Urn;
            }

            return Task.FromResult(new SetRootResourceResponse());
        }

        public Task<GetRootResourceResponse> GetRootResourceAsync(GetRootResourceRequest request)
        {
            lock (_rootResourceUrnLock)
            {
                if (_rootResourceUrn == null)
                    throw new InvalidOperationException("Root resource is not set");
                
                return Task.FromResult(new GetRootResourceResponse {Urn = _rootResourceUrn});
            }
        }
    }
}
