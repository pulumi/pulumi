// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

class MyComponent extends pulumi.ComponentResource {
    constructor(name: string) {
        super("test:index:MyComponent", name);
    }
}

pulumi.log.debug("A debug message");

new MyComponent("mycomponent");
