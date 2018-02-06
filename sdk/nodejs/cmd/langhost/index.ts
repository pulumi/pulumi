// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// This is the primary entrypoint for all Pulumi programs that are being watched by the resource planning
// monitor.  It creates the "host" that is responsible for wiring up gRPC connections to and from the monitor,
// and drives execution of a Node.js program, communicating back as required to track all resource allocations.

import * as minimist from "minimist";
import * as runtime from "../../runtime";

export function main(rawArgs: string[]): void {
    // Parse command line flags
    const argv: minimist.ParsedArgs = minimist(rawArgs, {
        string: [ "tracing" ],
    });

    // The program takes a single optional argument: the address of the engine endpoint.
    const args = argv._.slice(2);
    let serverAddr: string | undefined;
    if (args.length > 0) {
        serverAddr = args[0];
    }

    // Finally connect up the gRPC client/server and listen for incoming requests.
    const { server, port } = runtime.serveLanguageHost(serverAddr, argv["logging"]);

    // Emit the address so the monitor can read it to connect.  The gRPC server will keep the message loop alive.
    console.log(port);
}

main(process.argv);
