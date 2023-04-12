// Copyright 2016-2022, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class Resource extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:Resource", name, {}, opts);
    }
}

// Scenario #6 - Nested parents changing types
class ComponentSix extends pulumi.ComponentResource {
    resource: Resource;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:ComponentSix-v0", name, {}, opts);
        this.resource = new Resource("otherchild", {parent: this});
    }
}

class ComponentSixParent extends pulumi.ComponentResource {
    child: ComponentSix;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:ComponentSixParent-v0", name, {}, opts);
        this.child = new ComponentSix("child", {parent: this});
    }
}

const comp4 = new ComponentSixParent("comp6");
