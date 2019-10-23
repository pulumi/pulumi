// Copyright 2016-2019, Pulumi Corporation

module Program

open Pulumi.FSharp

[<EntryPoint>]
let main _ =
  Deployment.run Minimal.plain    
