using System.Threading.Tasks;

namespace Pulumi
{
    /// <summary>
    /// Metadata of the deployment that is currently running. Accessible via <see cref="Deployment.Instance"/>.
    /// </summary>
    public sealed class DeploymentInstance : IDeployment
    {
        private readonly IDeployment _deployment;
        
        internal DeploymentInstance(IDeployment deployment)
        {
            _deployment = deployment;
        }

        /// <summary>
        /// Returns the current stack name.
        /// </summary>
        public string StackName => _deployment.StackName;

        /// <summary>
        /// Returns the current project name.
        /// </summary>
        public string ProjectName => _deployment.ProjectName;

        /// <summary>
        /// Whether or not the application is currently being previewed or actually applied.
        /// </summary>
        public bool IsDryRun => _deployment.IsDryRun;

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
        public Task<T> InvokeAsync<T>(string token, InvokeArgs args, InvokeOptions? options = null)
            => _deployment.InvokeAsync<T>(token, args, options);

        /// <summary>
        /// Same as <see cref="InvokeAsync{T}(string, InvokeArgs, InvokeOptions)"/>, however the
        /// return value is ignored.
        /// </summary>
        public Task InvokeAsync(string token, InvokeArgs args, InvokeOptions? options = null)
            => _deployment.InvokeAsync(token, args, options);

        internal IDeploymentInternal Internal => (IDeploymentInternal)_deployment;
    }
}
