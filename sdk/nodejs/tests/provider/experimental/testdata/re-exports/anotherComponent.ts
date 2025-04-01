import * as pulumi from "@pulumi/pulumi";

export interface AnotherComponentArgs {
    count: number;
}

export class AnotherComponent extends pulumi.ComponentResource {
    public readonly result: pulumi.Output<number>;

    constructor(name: string, args: AnotherComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("test:index:AnotherComponent", name, {}, opts);
        this.result = pulumi.output(args.count).apply((n) => n * 2);
    }
}
