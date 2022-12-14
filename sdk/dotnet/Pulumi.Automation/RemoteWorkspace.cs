// Copyright 2016-2022, Pulumi Corporation

using System;
using System.Threading;
using System.Threading.Tasks;
using Pulumi.Automation.Commands;

namespace Pulumi.Automation
{
    public static class RemoteWorkspace
    {
        /// <summary>
        /// PREVIEW: Creates a Stack backed by a RemoteWorkspace with source code from the specified Git repository.
        /// Pulumi operations on the stack (Preview, Update, Refresh, and Destroy) are performed remotely.
        /// </summary>
        /// <param name="args">
        /// A set of arguments to initialize a RemoteStack with a remote Pulumi program from a Git repository.
        /// </param>
        public static Task<RemoteWorkspaceStack> CreateStackAsync(RemoteGitProgramArgs args)
            => CreateStackAsync(args, default);

        /// <summary>
        /// PREVIEW: Creates a Stack backed by a RemoteWorkspace with source code from the specified Git repository.
        /// Pulumi operations on the stack (Preview, Update, Refresh, and Destroy) are performed remotely.
        /// </summary>
        /// <param name="args">
        /// A set of arguments to initialize a RemoteStack with a remote Pulumi program from a Git repository.
        /// </param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public static Task<RemoteWorkspaceStack> CreateStackAsync(RemoteGitProgramArgs args, CancellationToken cancellationToken)
            => CreateStackHelperAsync(args, WorkspaceStack.CreateAsync, cancellationToken);

        /// <summary>
        /// PREVIEW: Selects an existing Stack backed by a RemoteWorkspace with source code from the specified Git
        /// repository. Pulumi operations on the stack (Preview, Update, Refresh, and Destroy) are performed remotely.
        /// </summary>
        /// <param name="args">
        /// A set of arguments to initialize a RemoteStack with a remote Pulumi program from a Git repository.
        /// </param>
        public static Task<RemoteWorkspaceStack> SelectStackAsync(RemoteGitProgramArgs args)
            => SelectStackAsync(args, default);

        /// <summary>
        /// PREVIEW: Selects an existing Stack backed by a RemoteWorkspace with source code from the specified Git
        /// repository. Pulumi operations on the stack (Preview, Update, Refresh, and Destroy) are performed remotely.
        /// </summary>
        /// <param name="args">
        /// A set of arguments to initialize a RemoteStack with a remote Pulumi program from a Git repository.
        /// </param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public static Task<RemoteWorkspaceStack> SelectStackAsync(RemoteGitProgramArgs args, CancellationToken cancellationToken)
            => CreateStackHelperAsync(args, WorkspaceStack.SelectAsync, cancellationToken);

        /// <summary>
        /// PREVIEW: Creates or selects an existing Stack backed by a RemoteWorkspace with source code from the specified
        /// Git repository. Pulumi operations on the stack (Preview, Update, Refresh, and Destroy) are performed remotely.
        /// </summary>
        /// <param name="args">
        /// A set of arguments to initialize a RemoteStack with a remote Pulumi program from a Git repository.
        /// </param>
        public static Task<RemoteWorkspaceStack> CreateOrSelectStackAsync(RemoteGitProgramArgs args)
            => CreateOrSelectStackAsync(args, default);

        /// <summary>
        /// PREVIEW: Creates or selects an existing Stack backed by a RemoteWorkspace with source code from the specified
        /// Git repository. Pulumi operations on the stack (Preview, Update, Refresh, and Destroy) are performed remotely.
        /// </summary>
        /// <param name="args">
        /// A set of arguments to initialize a RemoteStack with a remote Pulumi program from a Git repository.
        /// </param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public static Task<RemoteWorkspaceStack> CreateOrSelectStackAsync(RemoteGitProgramArgs args, CancellationToken cancellationToken)
            => CreateStackHelperAsync(args, WorkspaceStack.CreateOrSelectAsync, cancellationToken);

        private static async Task<RemoteWorkspaceStack> CreateStackHelperAsync(
            RemoteGitProgramArgs args,
            Func<string, Workspace, CancellationToken, Task<WorkspaceStack>> initFunc,
            CancellationToken cancellationToken)
        {
            if (!IsFullyQualifiedStackName(args.StackName))
            {
                throw new ArgumentException($"{nameof(args.StackName)} \"{args.StackName}\" not fully qualified.");
            }
            if (string.IsNullOrWhiteSpace(args.Url))
            {
                throw new ArgumentException($"{nameof(args.Url)} is required.");
            }
            if (!string.IsNullOrWhiteSpace(args.Branch) && !string.IsNullOrWhiteSpace(args.CommitHash))
            {
                throw new ArgumentException($"{nameof(args.Branch)} and {nameof(args.CommitHash)} cannot both be specified.");
            }
            if (string.IsNullOrWhiteSpace(args.Branch) && string.IsNullOrWhiteSpace(args.CommitHash))
            {
                throw new ArgumentException($"either {nameof(args.Branch)} or {nameof(args.CommitHash)} is required.");
            }
            if (!(args.Auth is null))
            {
                if (!string.IsNullOrWhiteSpace(args.Auth.SshPrivateKey) &&
                    !string.IsNullOrWhiteSpace(args.Auth.SshPrivateKeyPath))
                {
                    throw new ArgumentException($"{nameof(args.Auth.SshPrivateKey)} and {nameof(args.Auth.SshPrivateKeyPath)} cannot both be specified.");
                }
            }

            var localArgs = new LocalWorkspaceOptions
            {
                Remote = true,
                RemoteGitProgramArgs = args,
                RemoteEnvironmentVariables = args.EnvironmentVariables,
                RemotePreRunCommands = args.PreRunCommands,
            };

            var ws = new LocalWorkspace(
                new LocalPulumiCmd(),
                localArgs,
                cancellationToken);
            await ws.ReadyTask.ConfigureAwait(false);

            var stack = await initFunc(args.StackName, ws, cancellationToken).ConfigureAwait(false);
            return new RemoteWorkspaceStack(stack);
        }

        internal static bool IsFullyQualifiedStackName(string stackName)
        {
            if (string.IsNullOrWhiteSpace(stackName))
            {
                return false;
            }
            var split = stackName.Split("/");
            return split.Length == 3
                && !string.IsNullOrWhiteSpace(split[0])
                && !string.IsNullOrWhiteSpace(split[1])
                && !string.IsNullOrWhiteSpace(split[2]);
        }
    }
}
