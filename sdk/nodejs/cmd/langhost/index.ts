// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// This is the primary entrypoint for all Pulumi programs that are being watched by the resource planning
// monitor.  It creates the "host" that is responsible for wiring up gRPC connections to and from the monitor,
// and drives execution of a Node.js program, communicating back as required to track all resource allocations.

import * as minimist from "minimist";
import * as runtime from "../../runtime";

export function main(args: string[]): void {
    // The program requires a single argument: the address of the RPC endpoint for the resource monitor.  It
    // optionally also takes a second argument, a reference back to the engine, but this may be missing.
    if (args.length === 0) {
        console.error("fatal: Missing <monitor> address");
        process.exit(-1);
        return;
    }
    const monitorAddr: string = args[0];
    let serverAddr: string | undefined;
    if (args.length > 1) {
        serverAddr = args[1];
    }

    // Finally connect up the gRPC client/server and listen for incoming requests.
    const { server, port } = runtime.serveLanguageHost(monitorAddr, serverAddr);

    // Emit the address so the monitor can read it to connect.  The gRPC server will keep the message loop alive.
    console.log(port);
}

main(process.argv.slice(2));

