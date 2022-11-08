// Copyright 2016-2022, Pulumi Corporation

using System;
using Xunit;

namespace Pulumi.Automation.Tests
{
    public sealed class DeploymentsApiFactAttribute : FactAttribute
    {
        public DeploymentsApiFactAttribute()
        {
            if (Environment.GetEnvironmentVariable("PULUMI_ACCESS_TOKEN") is null)
            {
                Skip = "PULUMI_ACCESS_TOKEN not set";
            }
            else if (Environment.GetEnvironmentVariable("PULUMI_TEST_DEPLOYMENTS_API") is null)
            {
                Skip = "PULUMI_TEST_DEPLOYMENTS_API not set";
            }
        }
    }
}
