import * as pulumi from "@pulumi/pulumi";
import * as remote from "./remote";

interface RemoteMyComponentArgs {
    input1: pulumi.Input<number>;
}

// This is a proxy for the `MyComponent` defined in `./mycomponent`.  This wrapper could be
// auto-generated from a schematization of that component. It only requires:
//
// 1. The filesystem path to the NodeJS module to load
// 2. The (potentially qualified) global symbol to eval to get the instance of the component
//    constructor.
// 3. The input properties to define the `Args` interface above.
// 4. The output proeprties to define the public properties on this class.
//
// It works by remotely constructing the component in another NodeJS process (or in current
// implementation, VM inside the same process), serializing input properties (resolving inputs and
// promises) down to the standard Pulumi object model RPC, constructing the resources in the other
// process (which is connected to the same Pulumi engine host interfaces), then serializing the
// outputs of that component back through the Pulumi object model RPC into this process.  This
// remote invoke will only complete (asynchronously) once the remote component construction is
// complete (i.e. the `registerOutputs` has been called and it's output proeprties have resolved to
// concrete serializable values). The only return property we currently care about is the `urn`.  We
// can use this `urn` from within this process to go to the Pulumi engine and look up the output
// properties of the already-registered resource to populate this proxy resource.  This provides a
// lot of consistency with our existing `CustomResource` wrappers for both registered and read
// resources which populate proeprties based on outputs estabilshed by out-of-process RPCs.
//
// The end result is that constructing this component is observably "the same" as constucting the
// real component in the other language/runtime, with the exception that:
// 1. The properties of this resource are the registered outputs, not the host-language level
//    proeprties.  This ensures that a more clearly defined serializable subset of all host
//    langauges can be exposed.
class MyComponent extends pulumi.ComponentResource {
    public myid!: pulumi.Output<string>;
    public output1!: pulumi.Output<number>;
    constructor(name: string, args: RemoteMyComponentArgs, opts: pulumi.ComponentResourceOptions = {}) {
        // There are two cases:
        // 1. A URN was provided - in this case we are just going to look up the existing resource
        //    and populate this proxy from that URN.
        // 2. A URN was not provided - in this case we are going to remotely construct the resource,
        //    get the URN from the newly constructed resource, then look it up and populate this
        //    proxy from that URN.
        if (!opts.urn) {
            // TODO: Serialize `opts` to the remote construct
            const p = remote.construct("./mycomponent", "MyComponent", name, args /*,  opts */);
            const urn = p.then(r => <string>r.urn);
            opts = pulumi.mergeOptions(opts, { urn });
        }
        super("t", name, { ...args, myid: undefined, output1: undefined }, opts);
    }
}

const res = new MyComponent("n", {
    input1: Promise.resolve(24),
});

export const id2 = res.myid;
export const output1 = res.output1;
