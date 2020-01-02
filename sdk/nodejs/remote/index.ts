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

import * as cp from "child_process";
import * as grpc from "grpc";
import * as runtime from "../runtime";

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
    /* internal */
    private client: Promise<any>;
    /* internal */
    constructor() {
        const subprocess = cp.fork(require.resolve("./server"));
        // Ensure we can exit the current process without waiting on the VM server process to exit.
        subprocess.disconnect(); // detach the IPC connection
        subprocess.unref(); // do not track subprocess on our event loop
        this.client = new Promise(r => setTimeout(r, 1000)).then(() => {
            return new runtimeServiceProto.RuntimeClient(
                '0.0.0.0:50051',
                grpc.credentials.createInsecure()
            );
        });
    }

    public async construct(libraryPath: string, resource: string, name: string, args: any, opts?: any): Promise<any> {
        // TODO: Replace this with a proper wait on the server having launched (or retry).
        await new Promise(r => setTimeout(r, 1000));
        const serializedArgs = await runtime.serializeProperties("construct-args", args);
        const argsStruct = gstruct.Struct.fromJavaScript(serializedArgs);
        const serializedOpts = await runtime.serializeProperties("construct-opts", opts);
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
