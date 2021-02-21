// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation
{
    public class UpdateResult
    {
        public string StandardOutput { get; }

        public string StandardError { get; }

        public UpdateSummary Summary { get; }

        internal UpdateResult(
            string standardOutput,
            string standardError,
            UpdateSummary summary)
        {
            this.StandardOutput = standardOutput;
            this.StandardError = standardError;
            this.Summary = summary;
        }
    }
}
