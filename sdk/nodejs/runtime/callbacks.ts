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
import * as structproto from "google-protobuf/google/protobuf/struct_pb";
import * as log from "../log";
import * as callrpc from "../proto/callback_grpc_pb";
import * as callproto from "../proto/callback_pb";
import { Callback, CallbackInvokeRequest, CallbackInvokeResponse } from "../proto/callback_pb";
import * as resrpc from "../proto/resource_grpc_pb";
import { ResourceTransformation } from "../resource";

// maxRPCMessageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
/** @internal */
const maxRPCMessageSize: number = 1024 * 1024 * 400;

type CallbackFunction = (args: structproto.Value[]) => structproto.Value[];

export interface ICallbackServer {
    registerTransformation(callback: ResourceTransformation): Promise<callproto.Callback>;
    registerStackTransformation(callback: ResourceTransformation): void;
}

export class CallbackServer implements ICallbackServer {
    private readonly _callbacks = new Map<string, CallbackFunction>();
    private readonly _monitor: resrpc.IResourceMonitorClient;
    private readonly _server: grpc.Server;
    private readonly _target: Promise<string>;

    constructor(monitor: resrpc.IResourceMonitorClient) {
        this._monitor = monitor;

        this._server = new grpc.Server({
            "grpc.max_receive_message_length": maxRPCMessageSize,
        });

        const implementation: callrpc.ICallbacksServer = {
            invoke: this.invoke.bind(this),
        };
        this._server.addService(callrpc.CallbacksService, implementation);

        this._target = new Promise<string>((resolve, reject) => {
            this._server.bindAsync(`127.0.0.1:0`, grpc.ServerCredentials.createInsecure(), (err, port) => {
                if (err) {
                    reject(err);
                    return;
                }

                this._server.start();
                resolve(`127.0.0.1:${port}`);
            });
        });
    }

    private async invoke(
        call: grpc.ServerUnaryCall<CallbackInvokeRequest, CallbackInvokeResponse>,
        callback: grpc.sendUnaryData<CallbackInvokeResponse>,
    ) {
        const req = call.request;

        const cb = this._callbacks.get(req.getToken());
        if (cb === undefined) {
            const err = new grpc.StatusBuilder();
            err.withCode(grpc.status.INVALID_ARGUMENT);
            err.withDetails("callback not found");
            callback(err.build());
            return;
        }

        const resp = new CallbackInvokeResponse();
        try {
            resp.setReturnsList(cb(req.getArgumentsList()));
            callback(null, resp);
        } catch (e) {
            const err = new grpc.StatusBuilder();
            err.withCode(grpc.status.UNKNOWN);
            if (e instanceof Error) {
                err.withDetails(e.message);
            } else {
                err.withDetails(JSON.stringify(e));
            }
            callback(err.build());
            return;
        }
    }

    async registerTransformation(transform: ResourceTransformation): Promise<callproto.Callback> {
        const cb = (args: structproto.Value[]): structproto.Value[] => {
            return [];
        };
        const uuid = randomUUID();
        this._callbacks.set(uuid, cb);
        const req = new Callback();
        req.setToken(uuid);
        req.setTarget(await this._target);
        return req;
    }

    registerStackTransformation(transform: ResourceTransformation): void {
        this.registerTransformation(transform).then(
            (req) => {
                this._monitor.registerStackTransformation(req, (err, _) => {
                    if (err !== null) {
                        log.error(`failed to register stack transformation: ${err.message}`);
                        return;
                    }
                    // Remove this from the list of callbacks given we didn't manage to actually register it.
                    this._callbacks.delete(req.getToken());
                });
            },
            (err) => log.error(`failed to register stack transformation: ${err}`),
        );
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
