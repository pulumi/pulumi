import * as latestShipped from "@pulumi/pulumi";

export class ResourceClass extends latestShipped.ComponentResource {
    constructor(name: string, props: any, opts: latestShipped.ComponentResourceOptions) {
        super("", name, undefined, opts);
    }
}

export class ProviderMixin extends latestShipped.ComponentResource {
    constructor(name: string, props: any, opts: latestShipped.ComponentResourceOptions) {
        super("", name, undefined, opts);
    }
}
