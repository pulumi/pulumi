// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as child_process from "child_process";
import * as grpc from "grpc";
import * as readline from "readline";
import * as pulumi from "../";
import * as runtime from "../runtime";

//tslint:disable
const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");
const runtimeServiceProto = require("../proto/runtime_grpc_pb.js");
const runtimeProto = require("../proto/runtime_pb.js");

/**
 * ProxyComponentResource is the abstract base class for proxies around component resources.
 *
 * TODO: This should move into the core NodeJS SDK.
 */
export abstract class ProxyComponentResource extends pulumi.ComponentResource {
    constructor(
        t: string,
        name: string,
        libraryPath: string,
        libraryName: string,
        inputs: pulumi.Inputs,
        outputs: Record<string, undefined>,
        opts: pulumi.ComponentResourceOptions = {}) {
        // There are two cases:
        // 1. A URN was provided - in this case we are just going to look up the existing resource
        //    and populate this proxy from that URN.
        // 2. A URN was not provided - in this case we are going to remotely construct the resource,
        //    get the URN from the newly constructed resource, then look it up and populate this
        //    proxy from that URN.
        if (!opts.urn) {
            const p = getRemoteServer().construct(libraryPath, libraryName, name, inputs, opts);
            const urn = p.then(r => <string>r.urn);
            opts = pulumi.mergeOptions(opts, { urn });
        }
        const props = {
            ...inputs,
            ...outputs,
        };
        super(t, name, props, opts);
    }
}

let remoteServer: RemoteServer | undefined;
function getRemoteServer(): RemoteServer {
    if (!remoteServer) {
        remoteServer = new RemoteServer();
    }
    return remoteServer;
}

class RemoteServer {
    private readonly client: Promise<any>;
    constructor() {
        // Spawn a Node.js process to run a remote server.
        const subprocess = child_process.spawn(process.execPath, [require.resolve("./server")], {
            // Listen to stdout, ignore stdin and stderr.
            stdio: ["ignore", "pipe", "ignore"],
        });
        // Ensure we can exit the current process without waiting on the VM server process to exit.
        subprocess.unref(); // do not track subprocess on our event loop
        const reader = readline.createInterface(subprocess.stdout!);
        this.client = new Promise((resolve, reject) => {
            reader.once('line', port => {
                try {
                    // Tear down our piped stdout stream to that we do not hold the event loop open.
                    subprocess.stdout?.destroy();
                    // Connect to the process' gRPC server on the provided port.
                    const client = new runtimeServiceProto.RuntimeClient(
                        `0.0.0.0:${port}`,
                        grpc.credentials.createInsecure()
                    );
                    resolve(client);
                } catch (err) {
                    reject(err);
                }
            });
        });
    }

    public async construct(libraryPath: string, resource: string, name: string, args: any, opts?: any): Promise<any> {
        const serializedArgs = await runtime.serializeProperties("construct-args", args, { keepResources: true });
        const argsStruct = gstruct.Struct.fromJavaScript(serializedArgs);
        const serializedOpts = await runtime.serializeProperties("construct-opts", opts, { keepResources: true });
        const optsStruct = gstruct.Struct.fromJavaScript(serializedOpts);
        const client = await this.client;
        const constructRequest = new runtimeProto.ConstructRequest();
        constructRequest.setLibrarypath(libraryPath);
        constructRequest.setResource(resource);
        constructRequest.setName(name);
        constructRequest.setArgs(argsStruct);
        constructRequest.setOpts(optsStruct);
        const outsStruct = await new Promise<any>((resolve, reject) => {
            client.construct(constructRequest, (err: Error, resp: any) => {
                if (err) {
                    reject(err);
                } else {
                    resolve(resp.getOuts());
                }
            });
        });
        const outs = await runtime.deserializeProperties(outsStruct);
        return outs;
    }
}
