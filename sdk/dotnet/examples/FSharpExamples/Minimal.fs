namespace Pulumi.FSharpExamples

open System.Collections.Generic
open Pulumi
open Pulumi.Azure.Core
open Pulumi.Azure.Storage

module Minimal =

    let run (): IDictionary<string, Output<string>> =
                
        let resourceGroup = ResourceGroup("rg", ResourceGroupArgs(Location = input "West Europe"))

        let storageAccount =
            Account("sa", 
                AccountArgs(
                    ResourceGroupName = io resourceGroup.Name, // No implicit operators in F#
                    AccountReplicationType = input "LRS",      // Can't have two functions with same name but different signatures
                    AccountTier = input "Standard"))           // There may be some neat operator trick for that?

        dict [ ("accessKey", storageAccount.PrimaryAccessKey) ]
