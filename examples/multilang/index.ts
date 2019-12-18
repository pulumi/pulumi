import * as pulumi from "@pulumi/pulumi";
import * as remote from "./remote";

interface RemoteMyComponentArgs {
    input1: pulumi.Input<number>;
}

class RemoteMyComponent {
    public urn: pulumi.Output<string>;
    public id: pulumi.Output<string>;
    public output1: pulumi.Output<number>;
    constructor(name: string, args: RemoteMyComponentArgs) {
        const p = remote.construct("./mycomponent", "MyComponent", name, args);
        // TODO: This should have a dependency on the URN. 
        this.urn = pulumi.output(p.then(r => <string>r.urn));
        this.id = pulumi.output(p.then(r => <string>r.id));
        this.output1 = pulumi.output(p.then(r => <number>r.output1));
    }
}

const res = new RemoteMyComponent("n", {
    input1: Promise.resolve(42),
});

export const id = res.id;
export const output1 = res.output1;
