// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Linq;
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
            Assert.Matches(@"^(warning: A new version of Pulumi[^\n]+\n)?$",
                           result.StandardError);

            Assert.Equal(Lines(result.StandardOutput), stdoutLines);
            Assert.Equal(Lines(result.StandardError), stderrLines);
        }

        private IEnumerable<string> Lines(string s) {
            return s.Split(Environment.NewLine,
                           StringSplitOptions.RemoveEmptyEntries)
                .Select(x => x.Trim());
        }
    }
}
