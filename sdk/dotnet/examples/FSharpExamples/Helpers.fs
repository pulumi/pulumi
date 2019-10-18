namespace Pulumi

open Pulumi.Azure.Core
open Pulumi.Azure.Storage

[<AutoOpen>]
module Ops =
    let input<'a> (v: 'a): Input<'a> = Input.op_Implicit v
    let io<'a> (v: Output<'a>): Input<'a> = Input.op_Implicit v


[<AutoOpen>]
module Builders =

    type ResourceGroupBuilder internal (name) =
        member __.Yield(_) = ResourceGroupArgs()

        member __.Run(state: ResourceGroupArgs) : ResourceGroup = ResourceGroup(name, state)

        [<CustomOperation("location")>]
        member __.Location(state: ResourceGroupArgs, location: Input<string>) = 
            state.Location <- location
            state
        member this.Location(state: ResourceGroupArgs, location: Output<string>) = this.Location(state, io location)
        member this.Location(state: ResourceGroupArgs, location: string) = this.Location(state, input location)

    let resourceGroup name = ResourceGroupBuilder name

    type StorageAccountBuilder internal (name) =
        member __.Yield(_) = AccountArgs()

        member __.Run(state: AccountArgs) : Account = Account(name, state)

        [<CustomOperation("resourceGroupName")>]
        member __.ResourceGroupName(state: AccountArgs, value: Input<string>) = 
            state.ResourceGroupName <- value
            state
        member this.ResourceGroupName(state: AccountArgs, value: Output<string>) = this.ResourceGroupName(state, io value)
        member this.ResourceGroupName(state: AccountArgs, value: string) = this.ResourceGroupName(state, input value)

        [<CustomOperation("accountReplicationType")>]
        member __.AccountReplicationType(state: AccountArgs, value: Input<string>) = 
            state.AccountReplicationType <- value
            state
        member this.AccountReplicationType(state: AccountArgs, value: Output<string>) = this.AccountReplicationType(state, io value)
        member this.AccountReplicationType(state: AccountArgs, value: string) = this.AccountReplicationType(state, input value)

        [<CustomOperation("accountTier")>]
        member __.AccountTier(state: AccountArgs, value: Input<string>) = 
            state.AccountTier <- value
            state
        member this.AccountTier(state: AccountArgs, value: Output<string>) = this.AccountTier(state, io value)
        member this.AccountTier(state: AccountArgs, value: string) = this.AccountTier(state, input value)

    let storageAccount name = StorageAccountBuilder name