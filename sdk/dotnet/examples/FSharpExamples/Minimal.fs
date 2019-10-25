module Minimal

open System
open System.Collections.Generic
open Pulumi
open Pulumi.FSharp
open Pulumi.Azure.Core
open Pulumi.Azure.Storage

let plain (): IDictionary<string, Object> =                
    let resourceGroup = ResourceGroup("rg", ResourceGroupArgs(Location = input "WestEurope"))

    let storageAccount =
        Account("sa", 
            AccountArgs(
                ResourceGroupName = io resourceGroup.Name, // No implicit operators in F#
                AccountReplicationType = input "LRS",      // Can't have two functions with same name but different signatures
                AccountTier = input "Standard"))           // There may be some neat operator trick for that?

    dict [ ("accessKey", storageAccount.PrimaryAccessKey :> Object) ]


let ce (): IDictionary<string, Output<string>> =                
    let resourceGroup = resourceGroup "rg" {
        location "WestEurope"
    }

    let storageAccount = storageAccount "sa" {
        resourceGroupName resourceGroup.Name
        accountReplicationType "LRS"
        accountTier "Standard"
    }

    dict [ ("accessKey", storageAccount.PrimaryAccessKey) ]
