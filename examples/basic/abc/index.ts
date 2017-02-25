// Copyright 2016 Pulumi, Inc. All rights reserved.

import * as coconut from "@coconut/coconut";

class C extends coconut.Resource {
    constructor() {
        super();
    }
}

class B extends coconut.Resource {
    constructor() {
        super();
    }
}

class A extends coconut.Resource {
    private b: B;
    private c: C;

    constructor() {
        super();
        this.b = new B();
        this.c = new C();
    }
}

let a = new A();

