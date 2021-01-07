using System.Collections.Immutable;

namespace Pulumi.X.Automation
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
