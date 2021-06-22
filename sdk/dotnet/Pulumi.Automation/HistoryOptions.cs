// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation
{
    /// <summary>
    /// Options controlling the behavior of a <see cref="WorkspaceStack.GetHistoryAsync(HistoryOptions, System.Threading.CancellationToken)"/> operation.
    /// </summary>
    public sealed class HistoryOptions
    {
        public int? Page { get; set; }

        public int? PageSize { get; set; }
    }
}
