// This file is the stub Node entrypoint that must be spawned by any language which wants to host JS
// components.  It should most likely live inside the JS SDK (or another JS package).

const pulumi = require("@pulumi/pulumi");
const grpc = require("grpc");
const runtimeServiceProto = require("@pulumi/pulumi/proto/runtime_grpc_pb.js");
const runtimeProto = require("@pulumi/pulumi/proto/runtime_pb.js");
const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");

const server = new grpc.Server();
server.addService(runtimeServiceProto.RuntimeService, {
    construct: construct,
});
server.bind("0.0.0.0:50051", grpc.ServerCredentials.createInsecure());
server.start();

function construct(call, callback) {
    const library = require(call.request.getLibrarypath())
    const props = pulumi.runtime.deserializeProperties(call.request.getArgs());
    const opts = pulumi.runtime.deserializeProperties(call.request.getOpts());
    const res = new (library[call.request.getResource()])(call.request.getName(), props, opts);
    pulumi.runtime.serializeProperties("inner-construct", res).then(resolved => {
        return gstruct.Struct.fromJavaScript(resolved);
    }).then(outStruct => {
        const reply = new runtimeProto.ConstructResponse();
        reply.setOuts(outStruct);
        callback(null, reply)
    }, err => callback(err));
}
