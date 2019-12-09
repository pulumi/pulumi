// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Threading.Tasks;

namespace Pulumi
{
    internal interface IRunner
    {
        void RegisterTask(string description, Task task);
        Task<int> RunAsync(Func<Task<IDictionary<string, object?>>> func);
        Task<int> RunAsync<T>() where T : Stack, new();
    }
}
