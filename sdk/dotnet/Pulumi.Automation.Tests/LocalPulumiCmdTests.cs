// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Linq;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using Pulumi.Automation.Commands;
using Xunit;

namespace Pulumi.Automation.Tests
{
    public class LocalPulumiCmdTests
    {
        [Fact]
        public async Task CheckVersionCommand()
        {
            var localCmd = new LocalPulumiCmd();
            var extraEnv = new Dictionary<string, string?>();
            var args = new[] { "version" };

            var stdoutLines = new List<string>();
            var stderrLines = new List<string>();

            // NOTE: not testing onEngineEvent arg as that is not
            // supported for "version"; to test it one needs
            // workspace-aware commands such as up or preview;
            // currently this is covered by
            // LocalWorkspaceTests.HandlesEvents.

            var result = await localCmd.RunAsync(
                args, ".", extraEnv,
                onStandardOutput: line => stdoutLines.Add(line),
                onStandardError: line => stderrLines.Add(line));

            Assert.Equal(0, result.Code);

            Assert.Matches(@"^v?\d+\.\d+\.\d+", result.StandardOutput);
            // stderr must strictly begin with the version warning message or be an empty string:
            if (result.StandardError.Length > 0) {
                Assert.StartsWith("warning: A new version of Pulumi", result.StandardError);
            }

            // If these tests begin failing, it may be because the automation output now emits CRLF
            // (\r\n) on Windows.
            //
            // If so, update the Lines method to split on Environment.NewLine instead of "\n".
            Assert.Equal(Lines(result.StandardOutput), stdoutLines.Select(x => x.Trim()).ToList());
            Assert.Equal(Lines(result.StandardError), stderrLines.Select(x => x.Trim()).ToList());
        }

        private List<string> Lines(string s)
        {
            return s.Split("\n",
                           StringSplitOptions.RemoveEmptyEntries)
                .Select(x => x.Trim())
                .ToList();
        }
    }

}
