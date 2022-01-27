// Copyright 2016-2022, Pulumi Corporation

using Xunit;
using System.Collections.Generic;

namespace Pulumi.Tests.Resources
{
    public class MergeResourceOptionsTests
    {
        [Fact]
        public void MergeCustom()
        {
            var prov = new DependencyProviderResource("urn:pulumi:stack::project::pulumi:providers:aws::default_4_13_0");
            var o1 = new CustomResourceOptions {
                Provider = prov,
            };
            var o2 = new CustomResourceOptions {
                Protect = true,
            };
            var result = CustomResourceOptions.Merge(o1, o2);
            Assert.Equal("aws", result.Provider!.Package);
            Assert.True(result.Protect);
        }

        [Fact]
        public void MergeComponent()
        {
            var awsDefault = new DependencyProviderResource("urn:pulumi:stack::project::pulumi:providers:aws::default_4_13_0");
            var awsExplicit = new DependencyProviderResource("urn:pulumi:stack::project::pulumi:providers:aws::explicit");
            var azureDefault = new DependencyProviderResource("urn:pulumi:stack::project::pulumi:providers:azure::default_4_13_0");

            var o1 = new ComponentResourceOptions {
                Providers = new List<ProviderResource>{awsDefault, azureDefault},
                Protect = true,
            };

            var o2 = new ComponentResourceOptions {
                Providers = new List<ProviderResource>{awsExplicit},
                Protect = false,
            };

            var result = ComponentResourceOptions.Merge(o1, o2);
            Assert.False(result.Protect);
            Assert.Equal(azureDefault, result.Providers[0]);
            Assert.Equal(awsExplicit, result.Providers[1]);
            Assert.Equal(2, result.Providers.Count);
        }

        [Fact]
        public void MergeComponentEmpty()
        {
            var awsDefault = new DependencyProviderResource("urn:pulumi:stack::project::pulumi:providers:aws::default_4_13_0");
            var awsExplicit = new DependencyProviderResource("urn:pulumi:stack::project::pulumi:providers:aws::explicit");
            var azureDefault = new DependencyProviderResource("urn:pulumi:stack::project::pulumi:providers:azure::default_4_13_0");

            var o1 = new ComponentResourceOptions{
                Providers = new List<ProviderResource>{awsDefault, azureDefault},
                Provider = awsExplicit,
            };
            Assert.Equal(o1.Providers, ComponentResourceOptions.Merge(o1, null).Providers);
        }

        [Fact]
        public void MergeComponentSingleton()
        {
            var aws = new DependencyProviderResource("urn:pulumi:stack::project::pulumi:providers:aws::default_4_13_0");
            var o1 = new ComponentResourceOptions{
                Providers = new List<ProviderResource>{aws},
            };
            var o2 = new ComponentResourceOptions{
                Protect = true,
            };

            var result = ComponentResourceOptions.Merge(o1,o2);
            Assert.True(result.Protect);
            Assert.Null(result.Provider);
            Assert.Equal(aws, result.Providers[0]);
        }
    }
}
