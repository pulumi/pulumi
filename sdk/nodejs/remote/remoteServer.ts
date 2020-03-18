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
import * as runtime from "../runtime";
import * as settings from "../runtime/settings";

//tslint:disable
const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");
const runtimeServiceProto = require("../proto/runtime_grpc_pb.js");
const runtimeProto = require("../proto/runtime_pb.js");

let remoteServer: RemoteServer | undefined;
export function getRemoteServer(): RemoteServer {
    if (!remoteServer) {
        remoteServer = new RemoteServer();
    }
    return remoteServer;
}

export class RemoteServer {
    private readonly client: Promise<any>;
    constructor() {
        // Spawn a Node.js process to run a remote server.
        const subprocess = child_process.spawn(process.execPath, [require.resolve("./server")], {
            // Listen to stdout, ignore stdin and stderr.
            stdio: ["ignore", "pipe", "ignore"],
            env: {
                ...process.env,
                // We overwrite what we inherited from our own environment just in case any of these were
                // programmatically set during execution.  This is important for example if test mode was enabled
                // programatically.
                'PULUMI_NODEJS_PROJECT': settings.getProject(),
                'PULUMI_NODEJS_STACK': settings.getStack(),
                'PULUMI_NODEJS_DRY_RUN': settings.isDryRun() ? "true" : "false",
                'PULUMI_NODEJS_QUERY_MODE': settings.isQueryMode() ? "true": "false",
                'PULUMI_TEST_MODE': settings.isTestModeEnabled() ? "true" : "false",
            }
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
                    console.log("created client: " + JSON.stringify(client));
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
