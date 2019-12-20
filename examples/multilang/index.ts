import * as pulumi from "@pulumi/pulumi";
import * as remote from "./remote";

interface RemoteMyComponentArgs {
    input1: pulumi.Input<number>;
}

class MyComponent extends pulumi.ComponentResource {
    public myid!: pulumi.Output<string>;
    public output1!: pulumi.Output<number>;
    constructor(name: string, args: RemoteMyComponentArgs) {
        const p = remote.construct("./mycomponent", "MyComponent", name, args);
        const urn = p.then(r => <string>r.urn);
        super("t", name, { ...args, myid: undefined, output1: undefined }, { urn });
    }
}

const res2 = new MyComponent("n", {
    input1: Promise.resolve(24),
});

export const id2 = res2.myid;
