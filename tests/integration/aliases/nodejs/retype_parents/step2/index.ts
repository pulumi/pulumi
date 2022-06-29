// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class Resource extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:Resource", name, {}, opts);
    }
}

// Scenario #6 - Nested parents changing types
class ComponentSix extends pulumi.ComponentResource {

    private static generateAliases() : pulumi.Alias[] {
        const aliases: pulumi.Alias[] = [];
        for(let i = 0; i < 100; ++i) {
            aliases.push(
                { type: `my:module:ComponentSix-v${i}` }
            )
        }
        return aliases;
    }

    resource: Resource;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        // Add an alias that references the old type of this resource...
        const aliases = [...ComponentSix.generateAliases(), ...((opts && opts.aliases) || [])];
        // ..and then make the super call with the new type of this resource and the added alias.
        super("my:module:ComponentSix-v100", name, {}, { ...opts, aliases });
        // The child resource will also pick up an implicit alias due to the new type of the component it is parented
        // to.
        this.resource = new Resource("otherchild", { parent: this });
    }
}

class ComponentSixParent extends pulumi.ComponentResource {

    child: ComponentSix;

    private static generateAliases() : pulumi.Alias[] {
        const aliases: pulumi.Alias[] = [];
        for(let i = 0; i < 10; ++i) {
            aliases.push(
                { type: `my:module:ComponentSixParent-v${i}` }
            )
        }
        return aliases;
    }

    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        // Add an alias that references the old type of this resource...
        const aliases = [...ComponentSixParent.generateAliases(), ...((opts && opts.aliases) || [])];
        // ..and then make the super call with the new type of this resource and the added alias.
        super("my:module:ComponentSixParent-v10", name, {}, { ...opts, aliases });
        // The child resource will also pick up an implicit alias due to the new type of the component it is parented
        // to.
        this.child = new ComponentSix("child", { parent: this });
    }
}

const comp4 = new ComponentSixParent("comp6");
