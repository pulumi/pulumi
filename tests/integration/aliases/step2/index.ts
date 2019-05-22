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
// This resource was previously named `res1`, we'll alias to the old name.
const res1 = new Resource("newres1", {
    aliases: [pulumi.createUrn("res1", "my:module:Resource")],
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
            aliases: [pulumi.createUrn("res2", "my:module:Resource")],
        });
    }
}
// The creation of the component is unchanged.
const comp2 = new Component("comp2");

// Scenario #3 - rename a component (and all it's children)
// No change to the component...
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
// ...but applying an alias to the instance succesfully renames both the component and the children.
const comp3 = new ComponentThree("newcomp3", {
    aliases: [pulumi.createUrn("comp3", "my:module:ComponentThree")],
});

// Scenario #4 - change the type of a component
class ComponentFour extends pulumi.ComponentResource {
    resource: Resource;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        const aliases = (opts && opts.aliases) || [];
        aliases.push(pulumi.createUrn(name, "my:module:ComponentFour"));
        super("my:differentmodule:ComponentFourWithADifferentTypeName", name, {}, { ...opts, aliases });
        this.resource = new Resource("otherchild", {parent: this});
    }
}
const comp4 = new ComponentFour("comp4");

// Scenario #5 - composing #1 and #3
class ComponentFive extends pulumi.ComponentResource {
    resource: Resource;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:ComponentFive", name, {}, opts);
        this.resource = new Resource("otherchildrenamed", {
            parent: this,
            aliases: [ pulumi.createUrn("otherchild", "my:module:Resource", this)],
        });
    }
}
// ...but applying an alias to the instance succesfully renames both the component and the children.
const comp5 = new ComponentFive("newcomp5", {
    aliases: [pulumi.createUrn("comp5", "my:module:ComponentFive")],
});
