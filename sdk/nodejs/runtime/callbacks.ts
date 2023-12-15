// Copyright 2016-2023, Pulumi Corporation.
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

import * as grpc from "@grpc/grpc-js";
import { randomUUID } from "crypto";
import * as callrpc from "../proto/callback_grpc_pb";
import { Callback, CallbackInvokeRequest, CallbackInvokeResponse } from "../proto/callback_pb";
import * as resrpc from "../proto/resource_grpc_pb";
import { ResourceTransformation } from "../resource";

// maxRPCMessageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
/** @internal */
const maxRPCMessageSize: number = 1024 * 1024 * 400;

type CallbackFunction = (args: any) => any;

export interface ICallbackServer {
    registerStackTransformation(callback: ResourceTransformation): void;
}

class GrpcCallbackServer implements callrpc.ICallbacksServer{
    [name: string]: grpc.UntypedHandleCall;
    invoke: grpc.handleUnaryCall<CallbackInvokeRequest, CallbackInvokeResponse>;

}


export class CallbackServer implements ICallbackServer {
     private readonly _callbacks = new Map<string, CallbackFunction>();
     private readonly _monitor: resrpc.IResourceMonitorClient;
     private readonly _server: grpc.Server;
     private readonly _errors: grpc.ServiceError[] = [];

    constructor(monitor: resrpc.IResourceMonitorClient) {
        this._monitor = monitor;

        this._server = new grpc.Server({
            "grpc.max_receive_message_length": maxRPCMessageSize,
        });
        this._server.addService(callrpc.CallbacksService, this);
    }

    registerStackTransformation(callback: ResourceTransformation): void {
        throw new Error("Method not implemented.");

    }

    private registerCallback(callback: CallbackFunction): void {
        const uuid = randomUUID();
        this._callbacks.set(uuid, callback);
        const req = new Callback();
        this._monitor.registerStackTransformation(req, (err, res) => {
            if (err !== null) {
                this._errors.push(err);
            }
        })
    }
}

//
 // start() : Promise<void> {

 //     const port: number = await new Promise<number>((resolve, reject) => {
 //         server.bindAsync(`127.0.0.1:0`, grpc.ServerCredentials.createInsecure(), (err, p) => {
 //             if (err) {
 //                 reject(err);
 //             } else {
 //                 resolve(p);
 //             }
 //         });
 //     });
 //     server.start();
 // }
//export async function createCallbackService(monitor: resrpc.IResourceMonitorClient): Promise<CallbackServer> {
//    const server = new grpc.Server({
//        "grpc.max_receive_message_length": maxRPCMessageSize,
//    });
//    const calbackServer = new CallbackServer(monitor);
//    server.addService(provrpc.CallbacksService, calbackServer);
//    return calbackServer;
//   // onExit = (hasError: boolean) => {
//   //     languageServer.onPulumiExit(hasError);
//   //     server.forceShutdown();
//   // };
//}