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

import * as grpc from "grpc";
import * as runtime from "../../runtime";

//tslint:disable
const runtimeServiceProto = require("../../proto/runtime_grpc_pb.js");
const runtimeProto = require("../../proto/runtime_pb.js");
const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");

const server = new grpc.Server();
server.addService(runtimeServiceProto.RuntimeService, {
    construct: construct,
});
server.bind("0.0.0.0:50051", grpc.ServerCredentials.createInsecure());
server.start();

function construct(call: any, callback: (err: any, resp?: any) => void) {
    const library = require(call.request.getLibrarypath())
    const props = runtime.deserializeProperties(call.request.getArgs());
    const opts = runtime.deserializeProperties(call.request.getOpts());
    const res = new (library[call.request.getResource()])(call.request.getName(), props, opts);
    runtime.serializeProperties("inner-construct", res).then(resolved => {
        return gstruct.Struct.fromJavaScript(resolved);
    }).then(outStruct => {
        const reply = new runtimeProto.ConstructResponse();
        reply.setOuts(outStruct);
        callback(null, reply)
    }, err => callback(err));
}
