// Copyright 2016-2021, Pulumi Corporation

using System.Text;

namespace Pulumi.Automation.Commands
{
    internal class CommandResult
    {
        public int Code { get; }

        public string StandardOutput { get; }

        public string StandardError { get; }

        public CommandResult(
            int code,
            string standardOutput,
            string standardError)
        {
            this.Code = code;
            this.StandardOutput = standardOutput;
            this.StandardError = standardError;
        }

        public override string ToString()
        {
            var sb = new StringBuilder();
            sb.AppendLine($"code: {this.Code}");
            sb.AppendLine($"stdout: {this.StandardOutput}");
            sb.AppendLine($"stderr: {this.StandardError}");

            return sb.ToString();
        }
    }
}
