using System;
using System.Threading;
using System.Threading.Tasks;
using Pulumi.X.Automation.Commands.Exceptions;

namespace Pulumi.X.Automation
{
    public sealed class XStack // TODO: come up with a name for this
    {
        private readonly Task _readyTask;

        /// <summary>
        /// The name identifying the Stack.
        /// </summary>
        public string Name { get; }

        /// <summary>
        /// The Workspace the Stack was created from.
        /// </summary>
        public Workspace Workspace { get; }

        /// <summary>
        /// Creates a new stack using the given workspace, and stack name.
        /// It fails if a stack with that name already exists.
        /// </summary>
        /// <param name="name">The name identifying the stack.</param>
        /// <param name="workspace">The Workspace the Stack was created from.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        /// <exception cref="StackAlreadyExistsException">If a stack with the provided name already exists.</exception>
        public static async Task<XStack> CreateAsync(
            string name,
            Workspace workspace,
            CancellationToken cancellationToken = default)
        {
            var stack = new XStack(name, workspace, StackInitMode.Create, cancellationToken);
            await stack._readyTask;
            return stack;
        }

        /// <summary>
        /// Selects stack using the given workspace, and stack name.
        /// It returns an error if the given Stack does not exist.
        /// </summary>
        /// <param name="name">The name identifying the stack.</param>
        /// <param name="workspace">The Workspace the Stack was created from.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        /// <exception cref="StackNotFoundException">If a stack with the provided name does not exists.</exception>
        public static async Task<XStack> SelectAsync(
            string name,
            Workspace workspace,
            CancellationToken cancellationToken = default)
        {
            var stack = new XStack(name, workspace, StackInitMode.Select, cancellationToken);
            await stack._readyTask;
            return stack;
        }

        /// <summary>
        /// Tries to create a new Stack using the given workspace, and stack name
        /// if the stack does not already exist, or falls back to selecting an
        /// existing stack. If the stack does not exist, it will be created and
        /// selected.
        /// </summary>
        /// <param name="name">The name of the identifying stack.</param>
        /// <param name="workspace">The Workspace the Stack was created from.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public static async Task<XStack> CreateOrSelectAsync(
            string name,
            Workspace workspace,
            CancellationToken cancellationToken = default)
        {
            var stack = new XStack(name, workspace, StackInitMode.CreateOrSelect, cancellationToken);
            await stack._readyTask;
            return stack;
        }

        private XStack(
            string name,
            Workspace workspace,
            StackInitMode mode,
            CancellationToken cancellationToken)
        {
            this.Name = name;
            this.Workspace = workspace;

            switch (mode)
            {
                case StackInitMode.Create:
                    this._readyTask = workspace.CreateStackAsync(name, cancellationToken);
                    break;
                case StackInitMode.Select:
                    this._readyTask = workspace.SelectStackAsync(name, cancellationToken);
                    break;
                case StackInitMode.CreateOrSelect:
                    this._readyTask = Task.Run(async () =>
                    {
                        try
                        {
                            await workspace.CreateStackAsync(name, cancellationToken).ConfigureAwait(false);
                        }
                        catch (StackAlreadyExistsException)
                        {
                            await workspace.SelectStackAsync(name, cancellationToken).ConfigureAwait(false);
                        }
                    });
                    break;
                default:
                    throw new InvalidOperationException($"Unexpected Stack creation mode: {mode}");
            }
        }

        private enum StackInitMode // TODO: change name of this as per XStack
        {
            Create,
            Select,
            CreateOrSelect
        }
    }
}
