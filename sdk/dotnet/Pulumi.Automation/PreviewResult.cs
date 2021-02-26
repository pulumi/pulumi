// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation
{
    public class PreviewResult
    {
        public string StandardOutput { get; }

        public string StandardError { get; }

        internal PreviewResult(
            string standardOutput,
            string standardError)
        {
            this.StandardOutput = standardOutput;
            this.StandardError = standardError;
        }
    }
}
