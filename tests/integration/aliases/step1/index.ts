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
class ComponentThree extends pulumi.ComponentResource {
    resource1: Resource;
    resource2: Resource;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:ComponentThree", name, {}, opts);
        // Note that both un-prefixed and parent-name-prefixed child names are supported. For the later, the implicit
        // alias inherited from the parent alias will include replacing the name prefix to match the parent alias name.
        this.resource1 = new Resource(`${name}-child`, {parent: this});
        this.resource2 = new Resource("otherchild", {parent: this});
    }
}
const comp3 = new ComponentThree("comp3");

// Scenario #4 - change the type of a component
class ComponentFour extends pulumi.ComponentResource {
    resource: Resource;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:ComponentFour", name, {}, opts);
        this.resource = new Resource("otherchild", {parent: this});
    }
}
const comp4 = new ComponentFour("comp4");

// Scenario #5 - composing #1 and #3 and making both changes at the same time
class ComponentFive extends pulumi.ComponentResource {
    resource: Resource;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:ComponentFive", name, {}, opts);
        this.resource = new Resource("otherchild", {parent: this});
    }
}
const comp5 = new ComponentFive("comp5");
