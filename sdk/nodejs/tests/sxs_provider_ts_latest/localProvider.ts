import * as localUnshipped from "../../bin";

export class ResourceClass extends localUnshipped.ComponentResource {
    constructor(name: string, props: any, opts: localUnshipped.ComponentResourceOptions) {
        super("", name, undefined, opts);
    }
}

export class ProviderMixin extends localUnshipped.ComponentResource {
    constructor(name: string, props: any, opts: localUnshipped.ComponentResourceOptions) {
        super("", name, undefined, opts);
    }
}
