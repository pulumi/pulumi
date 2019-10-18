namespace Pulumi.FSharpExamples

open System.Collections.Generic
open Pulumi

module MinimalCE =

    let run (): IDictionary<string, Output<string>> =
                
        let resourceGroup = resourceGroup "rg" {
            location "WestEurope"
        }

        let storageAccount = storageAccount "sa" {
            resourceGroupName resourceGroup.Name
            accountReplicationType "LRS"
            accountTier "Standard"
        }

        dict [ ("accessKey", storageAccount.PrimaryAccessKey) ]
