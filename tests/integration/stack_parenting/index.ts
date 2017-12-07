// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as pulumi from "pulumi";

let currentID = 0;

class Provider implements pulumi.dynamic.ResourceProvider {
    public static instance = new Provider();

    public create: (inputs: any) => Promise<pulumi.dynamic.CreateResult>;

    constructor() {
        this.create = async (inputs: any) => {
            return {
                id: (currentID++).toString(),
                outs: undefined,
            };
        };
    }
}

class Resource extends pulumi.dynamic.Resource {
    constructor(name: string, parent?: pulumi.Resource) {
        super(Provider.instance, name, {}, parent, undefined);
    }
}

// Just allocate a few resources and make sure their URNs are correct with respect to parents, etc.  This
// should form a tree of roughly the following structure:
//
//     A      F
//    / \      \
//   B   C      G
//      / \
//     D   E
//
// with the caveat, of course, that A and F will share a common parent, the implicit stack.
let a = new Resource("a");

let b = new Resource("b", a);
let c = new Resource("c", a);

let d = new Resource("d", c);
let e = new Resource("e", c);

let f = new Resource("f");

let g = new Resource("g", f);
