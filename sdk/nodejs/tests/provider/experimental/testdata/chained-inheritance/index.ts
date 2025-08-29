import * as pulumi from "@pulumi/pulumi";

export interface MyComponentArgs {
    aNumber: pulumi.Input<number>;
    aString: pulumi.Input<string>;
}

export class MyComponent extends pulumi.ComponentResource {
    outNumber: pulumi.Output<number>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions, type: string = "provider:index:MyComponent") {
        super(type, name, args, opts);
    }
}

export class MyInheritingComponent extends MyComponent {
    outString: pulumi.Output<string>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super(name, args, opts, "provider:index:MyInheritingComponent");
    }
}
