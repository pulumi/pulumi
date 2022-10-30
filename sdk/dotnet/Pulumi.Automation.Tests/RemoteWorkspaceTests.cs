// Copyright 2016-2022, Pulumi Corporation

using Xunit;

namespace Pulumi.Automation.Tests
{
    public class RemoteWorkspaceTests
    {
        [Theory]
        [InlineData("owner/project/stack", true)]
        [InlineData("", false)]
        [InlineData("name", false)]
        [InlineData("owner/name", false)]
        [InlineData("/", false)]
        [InlineData("//", false)]
        [InlineData("///", false)]
        [InlineData("owner/project/stack/wat", false)]
        public void IsFullyQualifiedStackName(string input, bool expected)
        {
            var actual = RemoteWorkspace.IsFullyQualifiedStackName(input);
            Assert.Equal(expected, actual);
        }
    }
}
