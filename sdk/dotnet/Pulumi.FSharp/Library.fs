namespace Pulumi.FSharp

open System.Collections.Generic
open Pulumi

[<AutoOpen>]
module Ops =
    let input<'a> (v: 'a): Input<'a> = Input.op_Implicit v
    let io<'a> (v: Output<'a>): Input<'a> = Input.op_Implicit v

module Deployment = 
    let run (f: unit -> IDictionary<string, obj>) =
        Deployment.RunAsync (fun () -> f())
        |> Async.AwaitTask
        |> Async.RunSynchronously