// tslint:disable:file-header

import * as latestShipped from "@pulumi/pulumi";
import * as localUnshipped from "../../bin";

declare let latestShippedResource: latestShipped.Resource;
declare let localUnshippedResource: localUnshipped.Resource;

declare let latestShippedComponentResourceOptions: latestShipped.ComponentResourceOptions;
declare let localUnshippedComponentResourceOptions: localUnshipped.ComponentResourceOptions;

declare let latestShippedCustomResourceOptions: latestShipped.CustomResourceOptions;
declare let localUnshippedCustomResourceOptions: localUnshipped.CustomResourceOptions;

latestShippedResource = localUnshippedResource;
localUnshippedResource = latestShippedResource;

latestShippedComponentResourceOptions = localUnshippedComponentResourceOptions;
localUnshippedComponentResourceOptions = latestShippedComponentResourceOptions;

latestShippedCustomResourceOptions = localUnshippedCustomResourceOptions;
localUnshippedCustomResourceOptions = latestShippedCustomResourceOptions;

// simulate a resource similar to AWSX where there are instance methods that take
// other resources and options.

class LatestShippedDerivedComponentResourceExample extends latestShipped.ComponentResource {
    constructor(name: string, props: any, opts: latestShipped.ComponentResourceOptions) {
        super("", name, undefined, opts);
    }

    public createInstance(name: string, opts: latestShipped.ComponentResourceOptions): LatestShippedDerivedComponentResourceExample {
        throw new Error();
    }
}

class LocalUnshippedDerivedComponentResourceExample extends localUnshipped.ComponentResource {
    constructor(name: string, props: any, opts: localUnshipped.ComponentResourceOptions) {
        super("", name, undefined, opts);
    }

    public createInstance(name: string, opts: localUnshipped.ComponentResourceOptions): LocalUnshippedDerivedComponentResourceExample {
        throw new Error();
    }
}

declare let latestShippedComponentResource: LatestShippedDerivedComponentResourceExample;
declare let localUnshippedComponentResource: LocalUnshippedDerivedComponentResourceExample;

latestShippedComponentResource = localUnshippedComponentResource;
localUnshippedComponentResource = latestShippedComponentResource;
