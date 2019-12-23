using System.Collections.Generic;
using System.Threading.Tasks;

namespace Pulumi.Testing
{
    /// <summary>
    /// Testing context to pass into <see
    /// cref="Deployment.TestAsync{TStack}(ITestContext)"/>. Can be used to mock
    /// the behavior of deployment actions like resource registration and
    /// invokes.
    /// </summary>
    public interface ITestContext
    {
        /// <summary>
        /// Whether or not the application is currently being previewed or actually applied.
        /// </summary>
        bool IsDryRun { get; }

        /// <summary>
        /// Called when a new resource (including a stack) is registered.
        /// </summary>
        /// <param name="resource">A Pulumi resource (custom, component, stack).</param>
        /// <param name="args">Resource arguments.</param>
        /// <param name="options">Resource options.</param>
        void ReadOrRegisterResource(Resource resource, ResourceArgs args, ResourceOptions options);

        /// <summary>
        /// Called when resource outputs are registered.
        /// </summary>
        /// <param name="resource">A Pulumi resource (custom, component, stack).</param>
        /// <param name="outputs">A dictionary of output names and values.</param>
        void RegisterResourceOutputs(Resource resource, Output<IDictionary<string, object?>> outputs);

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
        Task<T> InvokeAsync<T>(string token, InvokeArgs args, InvokeOptions? options);

        /// <summary>
        /// Same as <see cref="InvokeAsync{T}(string, InvokeArgs, InvokeOptions)"/>, however the
        /// return value is ignored.
        /// </summary>
        Task InvokeAsync(string token, InvokeArgs args, InvokeOptions? options);
    }
}
