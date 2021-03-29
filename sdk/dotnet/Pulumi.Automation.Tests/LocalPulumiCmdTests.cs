namespace Pulumi.Automation.Tests
{
    using System.Collections.Generic;
    using System.Threading.Tasks;

    using Xunit;

    using Pulumi.Automation.Commands;

    public class LocalPulumiCmdTests
    {
        [Fact]
        public async Task CheckVersionCommand()
        {
            var localCmd = new LocalPulumiCmd();
            IDictionary<string,string> extraEnv = new Dictionary<string,string>();
            IEnumerable<string> args = new string[]{ "version" };
            var result = await localCmd.RunAsync(args, ".", extraEnv        
                // Uncommenting this currently fails the test on v2.23.1:
                //
                // Which versions of Pulumi CLI support --event-log param?
                //
                // , onEngineEvent: _ev => {} 
            );
            Assert.Equal(0, result.Code);
            Assert.Matches(@"^v\d+\.\d+\.\d+\n$", result.StandardOutput);
            Assert.Matches(@"^(warning: A new version of Pulumi[^\n]+\n)?$", result.StandardError);
        }
    }
}