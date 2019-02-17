// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class Resource extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:Resource", name, {}, opts);
    }
}

// Scenario #1 - rename a resource
const res1 = new Resource("res1");

// Scenario #2 - adopt a resource into a component
class Component extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:Component", name, {}, opts);
    }
}
const res2 = new Resource("res2");
const comp2 = new Component("comp2");

// Scenario #3 - rename a component (and all it's children)
class ComponentTwo extends pulumi.ComponentResource {
    resource: Resource;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:ComponentTwo", name, {}, opts);
        // TODO: Unfortunately, if this was names `${name}-child` which is best practice, it would not be possible to
        // rename the whole component, as both the parent structure and the name of the resource itself changes.
        this.resource = new Resource(`child`, {parent: this});
    }
}
const comp3 = new ComponentTwo("comp3");
