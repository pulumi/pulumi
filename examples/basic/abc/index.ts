// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as lumi from "@lumi/lumi";

class C extends lumi.Resource {
    constructor() {
        super();
    }
}

class B extends lumi.Resource {
    constructor() {
        super();
    }
}

class A extends lumi.Resource {
    private b: B;
    private c: C;

    constructor() {
        super();
        this.b = new B();
        this.c = new C();
    }
}

let a = new A();

