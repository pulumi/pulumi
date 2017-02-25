// Copyright 2016 Pulumi, Inc. All rights reserved.

import * as coconut from "@coconut/coconut";

class SimpleResource extends coconut.Resource {
    constructor() {
        super();
    }
}

let simple = new SimpleResource();

