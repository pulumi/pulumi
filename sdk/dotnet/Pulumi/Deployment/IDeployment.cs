// Copyright 2016-2019, Pulumi Corporation

using System.Threading.Tasks;

namespace Pulumi
{
    internal interface IDeployment
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
        /// Returns the current organization name.
        /// </summary>
        string OrganizationName { get; }

        /// <summary>
        /// Whether or not the application is currently being previewed or actually applied.
        /// </summary>
        bool IsDryRun { get; }

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
        Task<T> InvokeAsync<T>(string token, InvokeArgs args, InvokeOptions? options = null);

        /// <summary>
        /// Dynamically invokes the function '<paramref name="token"/>', which is offered by a
        /// provider plugin.
        /// <para/>
        /// The result of <see cref="InvokeSingleAsync"/> will be a <see cref="Task"/> resolved to the
        /// result value of the provider plugin that returns a bag of properties with a single value that is returned.
        /// <para/>
        /// The <paramref name="args"/> inputs can be a bag of computed values(including, `T`s,
        /// <see cref="Task{TResult}"/>s, <see cref="Output{T}"/>s etc.).
        /// </summary>
        Task<T> InvokeSingleAsync<T>(string token, InvokeArgs args, InvokeOptions? options = null);

        /// <summary>
        /// Dynamically invokes the function '<paramref name="token"/>', which is offered by a
        /// provider plugin.
        /// <para/>
        /// The result of <see cref="Invoke"/> will be a <see cref="Output"/> resolved to the
        /// result value of the provider plugin.
        /// <para/>
        /// The <paramref name="args"/> inputs can be a bag of computed values(including, `T`s,
        /// <see cref="Task{TResult}"/>s, <see cref="Output{T}"/>s etc.).
        /// </summary>
        Output<T> Invoke<T>(string token, InvokeArgs args, InvokeOptions? options = null);

        /// <summary>
        /// Dynamically invokes the function '<paramref name="token"/>', which is offered by a
        /// provider plugin.
        /// <para/>
        /// The result of <see cref="InvokeSingle"/> will be a <see cref="Output"/> resolved to the
        /// result value of the provider plugin that returns a bag of properties with a single value that is returned.
        /// <para/>
        /// The <paramref name="args"/> inputs can be a bag of computed values(including, `T`s,
        /// <see cref="Task{TResult}"/>s, <see cref="Output{T}"/>s etc.).
        /// </summary>
        Output<T> InvokeSingle<T>(string token, InvokeArgs args, InvokeOptions? options = null);

        /// <summary>
        /// Same as <see cref="InvokeAsync{T}(string, InvokeArgs, InvokeOptions)"/>, however the
        /// return value is ignored.
        /// </summary>
        Task InvokeAsync(string token, InvokeArgs args, InvokeOptions? options = null);

        /// <summary>
        /// Dynamically calls the function '<paramref name="token"/>', which is offered by a
        /// provider plugin. <see cref="Call{T}"/> returns immediately while the operation takes
        /// place asynchronously in the background, similar to Resource constructors.
        /// <para/>
        /// The result of <see cref="Call{T}"/> will be a <see cref="Output{T}"/> resolved to the
        /// result value of the provider plugin.
        /// <para/>
        /// The <paramref name="args"/> inputs can be a bag of computed values(including, `T`s,
        /// <see cref="Task{TResult}"/>s, <see cref="Output{T}"/>s etc.).
        /// </summary>
        Output<T> Call<T>(string token, CallArgs args, Resource? self = null, CallOptions? options = null);

        /// <summary>
        /// Same as <see cref="Call{T}"/>, however the return value is ignored.
        /// </summary>
        void Call(string token, CallArgs args, Resource? self = null, CallOptions? options = null);
    }
}
