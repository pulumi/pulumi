module Program

open System
open Pulumi.FSharp

let infra () =
  let config = new Pulumi.Config()
  let runtime = config.Get("runtime")
  Console.WriteLine("Hello from {0}", runtime)
  
  // Stack outputs
  dict []

[<EntryPoint>]
let main _ =
  Deployment.run infra
