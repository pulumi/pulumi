// tslint:disable:file-header

// See README.md for information on what to do if this test fails.

import * as latestProvider from "./latestProvider";
import * as localProvider from "./localProvider";

declare module "./latestProvider" {
    interface ProviderMixin {
        onEvent(name: string): ResourceClass;
    }
}

declare module "./localProvider" {
    interface ProviderMixin {
        onEvent(name: string): ResourceClass;
    }
}

latestProvider.ProviderMixin.prototype.onEvent = function(this: latestProvider.ProviderMixin, name) {
    return new latestProvider.ResourceClass(name, undefined, {});
}

localProvider.ProviderMixin.prototype.onEvent = function(this: localProvider.ProviderMixin, name) {
    return new localProvider.ResourceClass(name, undefined, {});
}

// make sure we can at least assign these to the Resource types from different versions.
declare let latestShippedDerivedComponentResource: latestProvider.ProviderMixin;
declare let localUnshippedDerivedComponentResource: localProvider.ProviderMixin;

localUnshippedDerivedComponentResource = latestShippedDerivedComponentResource;
