// Copyright 2016-2021, Pulumi Corporation

using System.Collections.Immutable;

namespace Pulumi.Automation
{
    public class PreviewResult
    {
        public string StandardOutput { get; }

        public string StandardError { get; }

        public IImmutableDictionary<OperationType, int> ChangeSummary { get; }

        internal PreviewResult(
            string standardOutput,
            string standardError,
            IImmutableDictionary<OperationType, int> changeSummary)
        {
            this.StandardOutput = standardOutput;
            this.StandardError = standardError;
            this.ChangeSummary = changeSummary;
        }
    }
}
