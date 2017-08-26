// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// This is the primary entrypoint for all Pulumi programs that are being watched by the resource planning
// monitor.  It creates the "host" that is responsible for wiring up gRPC connections to and from the monitor,
// and drives execution of a Node.js program, communicating back as required to track all resource allocations.

import * as minimist from "minimist";
import * as runtime from "../../lib/runtime";

export function main(args: string[]): void {
    // There are two command line flags:
    //     1) --monitor is the monitor's RPC interface we will communicate with.
    //     2) --port is the desired port for the host to listen on.  If blank, 0.0.0.0:0 will be used,
    //        which lets the kernel automatically choose a free port.  The program echoes the chosen port.
    let pargs: minimist.ParsedArgs = minimist(process.argv.slice(2), {
        string: [
            "monitor",
            "port",
        ],
        alias: {
            "m": "monitor",
            "p": "port",
        },
        unknown: (arg: string) => {
            if (arg[0] === "-") {
                console.error(`fatal: Unrecognized option '${arg}'`);
                process.exit(-1);
            }
            return true;
        },
    });

    // Ensure we have a monitor to connect to.
    let monitor: string | undefined = pargs["monitor"];
    if (!monitor) {
        console.error("fatal: Missing required flag --monitor");
        process.exit(-1);
        return;
    }

    // Now set up the server to listen for incoming requests, and spit out the resulting address.
    let port: number;
    if (pargs["port"]) {
        port = parseInt(pargs["port"], 10);
        if (isNaN(port)) {
            console.error(`fatal: Port ${pargs["port"]} is not a valid number`);
            process.exit(-1);
            return;
        }
    }
    else {
        port = 0;
    }

    // Finally connect up the gRPC client/server and listen for incoming requests.
    let { server, addr } = runtime.serveLanguageHost(monitor, port);

    // Emit the address so the monitor can read it to connect.  The gRPC server will keep the message loop alive.
    console.log(addr);
}

main(process.argv.slice(2));

