// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from "@mu/mu";

class C extends mu.Resource {
    constructor() {
        super();
    }
}

class B extends mu.Resource {
    constructor() {
        super();
    }
}

class A extends mu.Resource {
    private b: B;
    private c: C;

    constructor() {
        super();
        this.b = new B();
        this.c = new C();
    }
}

let a = new A();

