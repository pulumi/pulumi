using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;

namespace Pulumi.Automation
{
    /// <summary>
    /// Module class for manipulating stack state for a given <see cref="WorkspaceStack"/>.
    /// </summary>
    public sealed class WorkspaceStackState
    {
        private readonly WorkspaceStack _workspaceStack;

        internal WorkspaceStackState(WorkspaceStack workspaceStack)
        {
            this._workspaceStack = workspaceStack;
        }

        /// <summary>
        /// This command deletes a resource from a stack’s state, as long as it is safe to do so.
        /// The resource is specified by its Pulumi URN.
        /// <para/>
        /// Resources can’t be deleted if there exist other resources that depend on it or are parented to it.
        /// Protected resources will not be deleted unless it is specifically requested using the <paramref name="force"/> flag.
        /// </summary>
        /// <param name="urn">The Pulumi URN of the resource to be deleted.</param>
        /// <param name="force">A boolean indicating whether the deletion should be forced.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task DeleteAsync(string urn, bool force = false, CancellationToken cancellationToken = default)
        {
            var args = new List<string>()
            {
                "state",
                "delete",
                urn,
            };

            if (force)
                args.Add("--force");

            return this._workspaceStack.RunCommandAsync(args, null, null, null, cancellationToken);
        }

        /// <summary>
        /// Unprotect a resource in a stack's state.
        /// This command clears the ‘protect’ bit on the provided resource <paramref name="urn"/>, allowing the resource to be deleted.
        /// </summary>
        /// <param name="urn">The Pulumi URN to be unprotected.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task UnprotectAsync(string urn, CancellationToken cancellationToken = default)
        {
            var args = new string[] { "state", "unprotect", urn }.ToList();
            return this._workspaceStack.RunCommandAsync(args, null, null, null, cancellationToken);
        }

        /// <summary>
        /// Unprotect all resources in a stack's state.
        /// This command clears the ‘protect’ bit on all resources in the stack, allowing those resources to be deleted.
        /// </summary>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task UnprotectAllAsync(CancellationToken cancellationToken = default)
        {
            var args = new string[] { "state", "unprotect", "--all", }.ToList();
            return this._workspaceStack.RunCommandAsync(args, null, null, null, cancellationToken);
        }
    }
}
