// Copyright 2016-2022, Pulumi Corporation

namespace Pulumi
{
    public partial class Deployment
    {
        internal class RunnerOptions
        {
            /// <summary>
            /// Returns whether or not the runner is executing an inline program from the Automation API
            /// </summary>
            public bool IsInlineAutomationProgram { get; set; }
        }
    }
}