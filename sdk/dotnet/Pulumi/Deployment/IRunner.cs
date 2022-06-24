// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi
{
    internal interface IRunner
    {
        ImmutableList<Exception> SwallowedExceptions { get; }
        void RegisterTask(string description, Task task);
        Task<int> RunAsync(Func<Task<IDictionary<string, object?>>> func, StackOptions? options);
        Task<int> RunAsync<TStack>() where TStack : Stack, new();
        Task<int> RunAsync<TStack>(Func<TStack> stackFactory) where TStack : Stack;
        Task<int> RunAsync<TStack>(IServiceProvider serviceProvider) where TStack : Stack;
    }
}
