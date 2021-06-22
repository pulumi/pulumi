namespace Pulumi.FSharp

open Pulumi

[<AutoOpen>]
module Ops =

    /// <summary>
    /// Wraps a raw value into an <see cref="Input{'a}" />.
    /// </summary>
    let input<'a> (value: 'a): Input<'a> = Input.op_Implicit value

    /// <summary>
    /// Wraps an <see cref="Output" /> value into an <see cref="Input{'a}}" /> value.
    /// </summary>
    let io<'a> (v: Output<'a>): Input<'a> = Input.op_Implicit v

    /// <summary>
    /// Wraps a collection of items into an <see cref="InputList{'a}}" />.
    /// </summary>
    let inputList<'a> (items: seq<Input<'a>>) =
        let result = new InputList<'a>()
        for item in items do
            result.Add item
        result
    
    /// <summary>
    /// Wraps a collection of key-value pairs into an <see cref="InputMap{'a}}" />.
    /// </summary>
    let inputMap<'a> (items: seq<string * Input<'a>>) =
        let result = new InputMap<'a>()
        for (key, value) in items do
            result.Add(key, value)
        result

    /// <summary>
    /// Wraps a raw value for the first type into an <see cref="InputUnion{'a,'b}" />.
    /// </summary>
    let inputUnion1Of2<'a, 'b> (valA: 'a) = InputUnion<'a, 'b>.op_Implicit(valA)

    /// <summary>
    /// Wraps a raw value for the second type into an <see cref="InputUnion{'a,'b}" />.
    /// </summary>
    let inputUnion2Of2<'a, 'b> (valB: 'b) = InputUnion<'a, 'b>.op_Implicit(valB)


/// <summary>
/// Pulumi deployment functions.
/// </summary>
module Deployment =
    open System.Collections.Generic
    open System.Threading.Tasks
      
    /// <summary>
    /// Runs a task function as a Pulumi <see cref="Deployment" />.
    /// Blocks internally until the provided function completes,
    /// so that this function could be used directly from the main function.
    /// StackOptions can be provided to the deployment.
    /// </summary>
    let runTaskWithOptions (f: unit -> Task<IDictionary<string, obj>>) options =
        Deployment.RunAsync ((fun () -> f()), options)
        |> Async.AwaitTask
        |> Async.RunSynchronously

    /// <summary>
    /// Runs an async function as a Pulumi <see cref="Deployment" />.
    /// Blocks internally until the provided function completes,
    /// so that this function could be used directly from the main function.
    /// StackOptions can be provided to the deployment.
    /// </summary>
    let runAsyncWithOptions (f: unit -> Async<IDictionary<string, obj>>) options =
        runTaskWithOptions (f >> Async.StartAsTask) options

    /// <summary>
    /// Runs a task function as a Pulumi <see cref="Deployment" />.
    /// Blocks internally until the provided function completes,
    /// so that this function could be used directly from the main function.
    /// </summary>
    let runTask (f: unit -> Task<IDictionary<string, obj>>) =
        runTaskWithOptions f null
        
    /// <summary>
    /// Runs an async function as a Pulumi <see cref="Deployment" />.
    /// Blocks internally until the provided function completes,
    /// so that this function could be used directly from the main function.
    /// </summary>
    let runAsync (f: unit -> Async<IDictionary<string, obj>>) =
        runTask (f >> Async.StartAsTask)
        
    /// <summary>
    /// Runs a function as a Pulumi <see cref="Deployment" />.
    /// Blocks internally until the provided function completes,
    /// so that this function could be used directly from the main function.
    /// </summary>
    let run (f: unit -> IDictionary<string, obj>) =
        runTask (f >> Task.FromResult)

/// <summary>
/// Module containing utility functions to work with <see cref="Output{T}" />'s.
/// </summary>
module Outputs =
    
    /// <summary>
    /// Transforms the data of <see cref="Output{'a}"/> with the provided function <paramref
    /// name="f"/>. The result remains an <see cref="Output{'b}"/> so that dependent resources
    /// can be properly tracked.
    /// </summary>
    let apply<'a, 'b> (f: 'a -> 'b) (output: Output<'a>): Output<'b> =
        output.Apply f

    /// <summary>
    /// Transforms the data of <see cref="Output{'a}"/> with the provided asynchronous function <paramref
    /// name="f"/>. The result remains an <see cref="Output{'b}"/> so that dependent resources
    /// can be properly tracked.
    /// </summary>
    let applyAsync<'a, 'b> (f: 'a -> Async<'b>) (output: Output<'a>): Output<'b> =
        output.Apply<'b> (f >> Async.StartAsTask)

    /// <summary>
    /// Transforms the data of <see cref="Output{'a}"/> with the provided function <paramref
    /// name="f"/> that returns <see cref="Output{'b}"/>. The result is flattened to an <see cref="Output{'b}"/>.
    /// </summary>
    let bind<'a, 'b> (f: 'a -> Output<'b>) (output: Output<'a>): Output<'b> =
        output.Apply<'b> f
    
    /// <summary>
    /// Combines an <see cref="Output{'a}"/> with an <see cref="Output{'b}"/> to produce an <see cref="Output{'a*'b}"/>.
    /// </summary>
    let pair<'a, 'b> (a: Output<'a>) (b: Output<'b>): Output<'a * 'b> =
        Output.Tuple (a, b) 
        |> apply (fun struct (a, b) -> (a, b))

    /// <summary>
    /// Combines three values of <see cref="Output"/> to produce a three-way tuple <see cref="Output"/>.
    /// </summary>
    let pair3<'a, 'b, 'c> (a: Output<'a>) (b: Output<'b>) (c: Output<'c>): Output<'a * 'b * 'c> =
        Output.Tuple (io a, io b, io c) 
        |> apply (fun struct (a, b, c) -> (a, b, c))

    /// <summary>
    /// Combines four values of <see cref="Output"/> to produce a four-way tuple <see cref="Output"/>.
    /// </summary>
    let pair4<'a, 'b, 'c, 'd> (a: Output<'a>) (b: Output<'b>) (c: Output<'c>) (d: Output<'d>): Output<'a * 'b * 'c * 'd> =
        pair (pair a b) (pair c d)
        |> apply(fun ((a, b), (c, d)) -> a, b, c, d)

    /// <summary>
    /// Combines a list of <see cref="Output{'a}"/> to produce an <see cref="Output{List{'a}}"/>.
    /// </summary>
    let all<'a> (values: List<Output<'a>>): Output<List<'a>> = 
        Output.All (values |> List.map io |> List.toArray)
        |> apply List.ofSeq
