import { ComponentResource } from "@pulumi/pulumi";

interface OtherDeepComponentArgs {
    level: number;
    name?: string;
}

export class OtherDeepComponent extends ComponentResource {
    public readonly path: string;

    constructor(name: string, args: OtherDeepComponentArgs) {
        super("provider:index:OtherDeepComponent", name);
    }
}
