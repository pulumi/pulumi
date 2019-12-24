import * as pulumi from "@pulumi/pulumi";
import * as vm from "vm";

// This file would move into the core SDK.

//tslint:disable
const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");

// Simulates going through a full RPC call to another process: (1) serializing Inputs down to a
// protobuf (2) passing into a separate VM (3) deserializing inside the VM (4) constructing the
// resource inside the VM (5) serializing properties of the resource inside the VM (6)
// asynchronously returning this result from the VM (7) deserializing the returned properties and
// (8) returning these asynchronously to the caller.  The result is a flat JSON object representing
// the resources properties, which can be used to populate `Ouput`-typed properties on a proxy
// `Resource` object.
export async function construct(libraryPath: string, resource: string, name: string, args: any): Promise<any> {
    const resolved = await pulumi.runtime.serializeProperties("construct", args);
    const outputsObj = gstruct.Struct.fromJavaScript(resolved);

    let doneResolver: (value?: any) => void;
    let doneRejecter: (reason?: any) => void;
    const donePromise = new Promise<any>((resolve, reject) => { doneResolver = resolve; doneRejecter = reject})

    const sandbox = { 
        require: require, 
        console: console,
        _libraryPath: libraryPath, 
        _argsStruct: outputsObj, 
        _res: resource, 
        _name: name,
        _doneResolver: doneResolver!,
        _doneRejecter: doneRejecter!,
    };
    const context = vm.createContext(sandbox);
    vm.runInContext(`
(function() {
    var pulumi = require("@pulumi/pulumi");
    var gstruct = require("google-protobuf/google/protobuf/struct_pb.js");
    var library = require(_libraryPath)
    var args = pulumi.runtime.deserializeProperties(_argsStruct);
    const res = new (library[_res])(_name, args);
    return pulumi.runtime.serializeProperties("inner-construct", res).then(resolved => {
        //console.log(resolved);
        return gstruct.Struct.fromJavaScript(resolved);
    }).then(_doneResolver, _doneRejecter);
})()
`, context);

    const res = await donePromise;
    const outs = await pulumi.runtime.deserializeProperties(res);
    return outs;
}