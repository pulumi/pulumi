// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as dynamic from "@pulumi/pulumi/dynamic";
import { Component } from "./component";

let currentID = 0;

class Resource extends dynamic.Resource {
    constructor(name: string, opts?: pulumi.CustomResourceOptions) {
        const provider = {
            create: async (inputs: any) => ({
                id: (currentID++).toString(),
                outs: undefined,
            }),
        };

        super(provider, name, {}, opts);
    }
}

const resource = new Resource("resource");

const component = new Component("component", {
	message: resource.id.apply(v => `message ${v}`),
	nested: {
		value: resource.id.apply(v => `nested.value ${v}`),
	}
});
