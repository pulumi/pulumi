"use strict";
// tslint:disable:file-header
Object.defineProperty(exports, "__esModule", { value: true });
// See README.md for information on what to do if this test fails.
const latestShipped = require("@pulumi/pulumi");
// Note: we reference 'bin' as we want to see the typescript types with all internal information
// stripped.
const localUnshipped = require("../../bin");
latestShippedResource = localUnshippedResource;
localUnshippedResource = latestShippedResource;
latestShippedComponentResourceOptions = localUnshippedComponentResourceOptions;
localUnshippedComponentResourceOptions = latestShippedComponentResourceOptions;
latestShippedCustomResourceOptions = localUnshippedCustomResourceOptions;
localUnshippedCustomResourceOptions = latestShippedCustomResourceOptions;
// simulate a resource similar to AWSX where there are instance methods that take
// other resources and options.
class LatestShippedResourceExample1 extends latestShipped.ComponentResource {
    constructor(name, props, opts) {
        super("", name, undefined, opts);
    }
    createInstance(name, opts) {
        throw new Error();
    }
}
class LocalUnshippedResourceExample1 extends localUnshipped.ComponentResource {
    constructor(name, props, opts) {
        super("", name, undefined, opts);
    }
    createInstance(name, opts) {
        throw new Error();
    }
}
latestShippedResource = localUnshippedDerivedComponentResource;
localUnshippedResource = latestShippedDerivedComponentResource;
