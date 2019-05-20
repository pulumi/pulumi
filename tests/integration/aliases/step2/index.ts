// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

const stackName = pulumi.getStack();
const projectName = pulumi.getProject();

class Resource extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:Resource", name, {}, opts);
    }
}

// Scenario #1 - rename a resource
// This resource was previously named `one`, we'll alias to the old name.
const res1 = new Resource("newres1", {
    aliases: [`urn:pulumi:${stackName}::${projectName}::my:module:Resource::res1`],
});

// Scenario #2 - adopt a resource into a component The component author is the same as the component user, and changes
// the component to be able to adopt the resource that was previously defined separately...
class Component extends pulumi.ComponentResource {
    resource: Resource;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:Component", name, {}, opts);
        // The resource creation was moved from top level to inside the component.
        this.resource = new Resource(`${name}-child`, {
            // With a new parent
            parent: this,
            // But with an alias provided based on knowing where the resource existing before - in this case at top
            // level.
            aliases: [`urn:pulumi:${stackName}::${projectName}::my:module:Resource::res2`],
        });
    }
}
// The creation of the component is unchanged.
const comp2 = new Component("comp2");

// Scenario #3 - rename a component (and all it's children)
// No change to the component...
class ComponentTwo extends pulumi.ComponentResource {
    resource1: Resource;
    resource2: Resource;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:ComponentTwo", name, {}, opts);
        // Note that both un-prefixed and parent-name-prefixed child names are supported. For the later, the implicit
        // alias inherited from the parent alias will include replacing the name prefix to match the parent alias name.
        this.resource1 = new Resource(`${name}-child`, {parent: this});
        this.resource2 = new Resource("otherchild", {parent: this});
    }
}
// ...but applying an alias to the instance succesfully renames both the component and the children.
const comp3 = new ComponentTwo("newcomp3", {
    aliases: [`urn:pulumi:${stackName}::${projectName}::my:module:ComponentTwo::comp3`],
});


