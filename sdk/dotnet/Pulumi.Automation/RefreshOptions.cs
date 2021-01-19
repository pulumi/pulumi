using System;

namespace Pulumi.Automation
{
    /// <summary>
    /// Options controlling the behavior of an <see cref="XStack.RefreshAsync(RefreshOptions, System.Threading.CancellationToken)"/> operation.
    /// </summary>
    public sealed class RefreshOptions : UpdateOptions
    {
        public bool? ExpectNoChanges { get; set; }

        public Action<string>? OnOutput { get; set; }
    }
}
