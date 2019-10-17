// tslint:disable:file-header

// See README.md for information on what to do if this test fails.

import * as latestShipped from "@pulumi/pulumi";

// Note: we reference 'bin' as we want to see the typescript types with all internal information
// stripped.
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

class LatestShippedResourceExample1 extends latestShipped.ComponentResource {
    constructor(name: string, props: any, opts: latestShipped.ComponentResourceOptions) {
        super("", name, undefined, opts);
    }

    public createInstance(name: string, opts: latestShipped.ComponentResourceOptions): LatestShippedResourceExample1 {
        throw new Error();
    }
}

class LocalUnshippedResourceExample1 extends localUnshipped.ComponentResource {
    constructor(name: string, props: any, opts: localUnshipped.ComponentResourceOptions) {
        super("", name, undefined, opts);
    }

    public createInstance(name: string, opts: localUnshipped.ComponentResourceOptions): LocalUnshippedResourceExample1 {
        throw new Error();
    }
}

// make sure we can at least assign these to the Resource types from different versions.
declare let latestShippedDerivedComponentResource: LatestShippedResourceExample1;
declare let localUnshippedDerivedComponentResource: LocalUnshippedResourceExample1;

latestShippedResource = localUnshippedDerivedComponentResource;
localUnshippedResource = latestShippedDerivedComponentResource;
