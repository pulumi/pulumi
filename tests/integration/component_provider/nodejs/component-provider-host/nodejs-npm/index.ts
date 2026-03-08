// Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as provider from "@pulumi/nodejs-component-provider";
// TODO: https://github.com/pulumi/pulumi/issues/20252
// For the resource reference to `random.RandomPet` to work, we need to ensure
// the `@pulumi/random` package is imported. As a side effect of the import, we
// call `registerResourceModule`, which is needed for the deserialization code
// to lookup the constructor for given URN.
import * as random from "@pulumi/random"
import * as assert from "node:assert";
assert(random);

let parent = new pulumi.ComponentResource("ParentComponent", "parent");

let comp = new provider.MyComponent("comp", {
    aNumber: 123,
    anOptionalString: "Bonnie",
    aBooleanInput: pulumi.Output.create(true),
    aComplexTypeInput: {
        aNumber: 7,
        nestedComplexType: {
            aNumber: 9,
        }
    },
    enumInput: provider.MyEnum.B,
}, { parent: parent })

export const urn = comp.urn;
export const aNumberOutput = comp.aNumberOutput;
export const anOptionalStringOutput = comp.anOptionalStringOutput;
export const aBooleanOutput = comp.aBooleanOutput;
export const aComplexTypeOutput = comp.aComplexTypeOutput;
export const aResourceOutputUrn = comp.aResourceOutput.urn;
export const aString = comp.aString;
export const enumOutput = comp.enumOutput;
