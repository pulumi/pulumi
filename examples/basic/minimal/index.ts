// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as lumi from "@lumi/lumi";

class SimpleResource extends lumi.Resource {
    constructor() {
        super();
    }
}

let simple = new SimpleResource();

