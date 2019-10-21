// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi
{
    public interface IDeployment
    {
        /// <summary>
        /// Returns the current stack name.
        /// </summary>
        string StackName { get; }

        /// <summary>
        /// Returns the current project name.
        /// </summary>
        string ProjectName { get; }

        /// <summary>
        /// Whether or not the application is currently being previewed or actually applied.
        /// </summary>
        bool IsDryRun { get; }

        /// <summary>
        /// The Stack that the deployment is producing.
        /// </summary>
        Stack Stack { get; }

        /// <summary>
        /// Dynamically invokes the function '<paramref name="token"/>', which is offered by a
        /// provider plugin.
        /// <para/>
        /// The result of <see cref="InvokeAsync"/> will be a <see cref="Task"/> resolved to the
        /// result value of the provider plugin.
        /// <para/>
        /// The <paramref name="args"/> inputs can be a bag of computed values(including, `T`s,
        /// <see cref="Task{TResult}"/>s, <see cref="Output{T}"/>s etc.).
        /// </summary>
        Task<T> InvokeAsync<T>(
            string token, ResourceArgs args,
            Func<ImmutableDictionary<string, object>, T> convert, InvokeOptions? options = null);
    }
}
