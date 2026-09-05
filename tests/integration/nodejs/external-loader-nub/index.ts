// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import { suffix } from "./shared.js";

const provider: pulumi.dynamic.ResourceProvider = {
    async create(inputs) {
        return { id: `dynamic-${inputs.value}`, outs: { value: `${inputs.value}${suffix}` } };
    },
};

class DynamicResource extends pulumi.dynamic.Resource {
    declare readonly value: pulumi.Output<string>;

    constructor(name: string, props: pulumi.Inputs) {
        super(provider, name, props);
    }
}

const resource = new DynamicResource("resource", { value: "value" });
const serialized = pulumi.runtime.serializeFunction(() => suffix);

export const dynamicId = resource.id;
export const dynamicValue = resource.value;
export const serializedLength = pulumi.output(serialized).apply(value => value.text.length);
