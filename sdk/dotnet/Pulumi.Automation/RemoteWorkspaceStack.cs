// Copyright 2016-2022, Pulumi Corporation

using System;
using System.Collections.Immutable;
using System.Threading;
using System.Threading.Tasks;

namespace Pulumi.Automation
{
    public sealed class RemoteWorkspaceStack : IDisposable
    {
        private readonly WorkspaceStack _stack;

        /// <summary>
        /// The name identifying the Stack.
        /// </summary>
        public string Name => _stack.Name;

        internal RemoteWorkspaceStack(WorkspaceStack stack)
        {
            _stack = stack;
        }

        /// <summary>
        /// Creates or updates the resources in a stack by executing the program in the Workspace.
        /// This operation runs remotely.
        /// <para/>
        /// https://www.pulumi.com/docs/reference/cli/pulumi_up/
        /// </summary>
        /// <param name="options">Options to customize the behavior of the update.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task<UpResult> UpAsync(
            RemoteUpOptions? options = null,
            CancellationToken cancellationToken = default)
        {
            var upOptions = new UpOptions
            {
                OnStandardOutput = options?.OnStandardOutput,
                OnStandardError = options?.OnStandardError,
                OnEvent = options?.OnEvent,
            };
            return _stack.UpAsync(upOptions, cancellationToken);
        }

        /// <summary>
        /// Performs a dry-run update to a stack, returning pending changes.
        /// This operation runs remotely.
        /// <para/>
        /// https://www.pulumi.com/docs/reference/cli/pulumi_preview/
        /// </summary>
        /// <param name="options">Options to customize the behavior of the update.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task<PreviewResult> PreviewAsync(
            RemotePreviewOptions? options = null,
            CancellationToken cancellationToken = default)
        {
            var previewOptions = new PreviewOptions
            {
                OnStandardOutput = options?.OnStandardOutput,
                OnStandardError = options?.OnStandardError,
                OnEvent = options?.OnEvent,
            };
            return _stack.PreviewAsync(previewOptions, cancellationToken);
        }

        /// <summary>
        /// Compares the current stack’s resource state with the state known to exist in the actual
        /// cloud provider. Any such changes are adopted into the current stack.
        /// This operation runs remotely.
        /// </summary>
        /// <param name="options">Options to customize the behavior of the refresh.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task<UpdateResult> RefreshAsync(
            RemoteRefreshOptions? options = null,
            CancellationToken cancellationToken = default)
        {
            var refreshOptions = new RefreshOptions
            {
                OnStandardOutput = options?.OnStandardOutput,
                OnStandardError = options?.OnStandardError,
                OnEvent = options?.OnEvent,
            };
            return _stack.RefreshAsync(refreshOptions, cancellationToken);
        }

        /// <summary>
        /// Destroy deletes all resources in a stack, leaving all history and configuration intact.
        /// This operation runs remotely.
        /// </summary>
        /// <param name="options">Options to customize the behavior of the destroy.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task<UpdateResult> DestroyAsync(
            RemoteDestroyOptions? options = null,
            CancellationToken cancellationToken = default)
        {
            var destroyOptions = new DestroyOptions
            {
                OnStandardOutput = options?.OnStandardOutput,
                OnStandardError = options?.OnStandardError,
                OnEvent = options?.OnEvent,
            };
            return _stack.DestroyAsync(destroyOptions, cancellationToken);
        }

        /// <summary>
        /// Gets the current set of Stack outputs from the last <see cref="UpAsync(RemoteUpOptions?, CancellationToken)"/>.
        /// </summary>
        public Task<ImmutableDictionary<string, OutputValue>> GetOutputsAsync(CancellationToken cancellationToken = default)
            => _stack.GetOutputsAsync(cancellationToken);

        /// <summary>
        /// Returns a list summarizing all previews and current results from Stack lifecycle operations (up/preview/refresh/destroy).
        /// </summary>
        /// <param name="options">Options to customize the behavior of the fetch history action.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task<ImmutableList<UpdateSummary>> GetHistoryAsync(
            HistoryOptions? options = null,
            CancellationToken cancellationToken = default)
        {
            return _stack.GetHistoryAsync(options, cancellationToken);
        }

        /// <summary>
        /// Exports the deployment state of the stack.
        /// <para/>
        /// This can be combined with ImportStackAsync to edit a
        /// stack's state (such as recovery from failed deployments).
        /// </summary>
        public Task<StackDeployment> ExportStackAsync(CancellationToken cancellationToken = default)
            => _stack.ExportStackAsync(cancellationToken);

        /// <summary>
        /// Imports the specified deployment state into a pre-existing stack.
        /// <para/>
        /// This can be combined with ExportStackAsync to edit a
        /// stack's state (such as recovery from failed deployments).
        /// </summary>
        public Task ImportStackAsync(StackDeployment state, CancellationToken cancellationToken = default)
            => _stack.ImportStackAsync(state, cancellationToken);

        /// <summary>
        /// Cancel stops a stack's currently running update. It throws
        /// an exception if no update is currently running. Note that
        /// this operation is _very dangerous_, and may leave the
        /// stack in an inconsistent state if a resource operation was
        /// pending when the update was canceled. This command is not
        /// supported for local backends.
        /// </summary>
        public Task CancelAsync(CancellationToken cancellationToken = default)
            => _stack.CancelAsync(cancellationToken);

        public void Dispose()
            => _stack.Dispose();
    }
}
