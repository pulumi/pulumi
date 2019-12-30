import * as pulumi from "@pulumi/pulumi";
import * as cp from "child_process";

// This file would move into the core SDK.

//tslint:disable
const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");
const runtimeServiceProto = require("@pulumi/pulumi/proto/runtime_grpc_pb.js");
const runtimeProto = require("@pulumi/pulumi/proto/runtime_pb.js");
// TODO: Apparently grpc can't sxs - so we must pick up the version that matches what the service
// above will use, which when linking will be it's local `node_modules` version.
const grpc = require("@pulumi/pulumi/node_modules/grpc")

function spawnServerVM() {
    const subprocess = cp.fork(require.resolve("../server"));
    // Ensure we can exit the current process without waiting on the VM server process to exit.
    subprocess.disconnect(); // detach the IPC connection
    subprocess.unref(); // do not track subprocess on our event loop
}

spawnServerVM()

export async function construct(libraryPath: string, resource: string, name: string, args: any, opts?: any): Promise<any> {
    // TODO: Replace this with a proper wait on the server having launched (or retry).
    await new Promise(r => setTimeout(r, 1000));
    const serializedArgs = await pulumi.runtime.serializeProperties("construct-args", args);
    const argsStruct = gstruct.Struct.fromJavaScript(serializedArgs);
    const serializedOpts = await pulumi.runtime.serializeProperties("construct-opts", opts);
    const optsStruct = gstruct.Struct.fromJavaScript(serializedOpts);
    const outsStruct = await new Promise<any>((resolve, reject) => {
        const client = new runtimeServiceProto.RuntimeClient('0.0.0.0:50051', grpc.credentials.createInsecure());
        const constructRequest = new runtimeProto.ConstructRequest();
        constructRequest.setLibrarypath(libraryPath);
        constructRequest.setResource(resource);
        constructRequest.setName(name);
        constructRequest.setArgs(argsStruct);
        constructRequest.setOpts(optsStruct);
        client.construct(constructRequest, (err: Error, resp: any) => {
            if (err) {
                reject(err);
            } else {
                resolve(resp.getOuts());
            }
        });
    });

    const outs = await pulumi.runtime.deserializeProperties(outsStruct);
    return outs;
}
