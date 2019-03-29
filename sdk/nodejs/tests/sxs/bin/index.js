"use strict";
// tslint:disable:file-header
Object.defineProperty(exports, "__esModule", { value: true });
const latestShipped = require("@pulumi/pulumi");
const localUnshipped = require("../../bin");
latestShippedResource = localUnshippedResource;
localUnshippedResource = latestShippedResource;
latestShippedComponentResourceOptions = localUnshippedComponentResourceOptions;
localUnshippedComponentResourceOptions = latestShippedComponentResourceOptions;
latestShippedCustomResourceOptions = localUnshippedCustomResourceOptions;
localUnshippedCustomResourceOptions = latestShippedCustomResourceOptions;
// simulate a resource similar to AWSX where there are instance methods that take
// other resources and options.
class LatestShippedDerivedComponentResourceExample extends latestShipped.ComponentResource {
    constructor(name, props, opts) {
        super("", name, undefined, opts);
    }
    createInstance(name, opts) {
        throw new Error();
    }
}
class LocalUnshippedDerivedComponentResourceExample extends localUnshipped.ComponentResource {
    constructor(name, props, opts) {
        super("", name, undefined, opts);
    }
    createInstance(name, opts) {
        throw new Error();
    }
}
latestShippedComponentResource = localUnshippedComponentResource;
localUnshippedComponentResource = latestShippedComponentResource;
