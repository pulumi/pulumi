// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as lumi from "@lumi/lumi";

class SimpleResource extends lumi.Resource {
    constructor() {
        super();
    }
}

let simple = new SimpleResource();

