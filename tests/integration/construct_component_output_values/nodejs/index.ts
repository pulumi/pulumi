// Copyright 2016, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

import { Component } from "./component";

class Dependency extends pulumi.CustomResource {
    constructor(name: string) {
        super("testprovider:index:Random", name, { length: 1 });
    }
}

const dep = new Dependency("dep");
const b = dep.urn.apply(_ => "shh");

new Component("component", {
    foo: {
        something: "hello",
    },
    bar: {
        tags: {
            "a": "world",
            "b": b,
        },
    },
});
