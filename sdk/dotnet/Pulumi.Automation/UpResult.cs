// Copyright 2016-2021, Pulumi Corporation

using System.Collections.Immutable;

namespace Pulumi.Automation
{
    public sealed class UpResult : UpdateResult
    {
        public IImmutableDictionary<string, OutputValue> Outputs { get; }

        internal UpResult(
            string standardOutput,
            string standardError,
            UpdateSummary summary,
            IImmutableDictionary<string, OutputValue> outputs)
            : base(standardOutput, standardError, summary)
        {
            this.Outputs = outputs;
        }
    }
}
