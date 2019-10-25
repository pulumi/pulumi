// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System.Collections.Generic;
using Pulumi.Azure.Core;
using Storage = Pulumi.Azure.Storage;

namespace Pulumi.CSharpExamples
{
    public class Minimal
    {
        public static IDictionary<string, Output<string>> Run()
        {
            var resourceGroup = new ResourceGroup("rg", new ResourceGroupArgs { Location = "West Europe" });

            // "Account" without a namespace would be too vague, while "ResourceGroup" without namespace sounds good.
            // We could suggest always using the namespace, but this makes new-ing of Args even longer and uglier?
            var storageAccount = new Storage.Account("sa", new Storage.AccountArgs
            {
                ResourceGroupName = resourceGroup.Name,
                AccountReplicationType = "LRS",
                AccessTier = "Standard",
            });

            // How do we want to treat exports?
            return new Dictionary<string, Output<string>>
            {
                { "accessKey", storageAccount.PrimaryAccessKey }
            };
        }
    }
}
