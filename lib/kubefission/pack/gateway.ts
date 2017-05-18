// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as kubernetes from "@lumi/kubernetes";

export class Gateway {
    constructor() {
        this.fns = new kubernetes.Namespace({
            name: "fission",
            labels: { name: "fission" },
        });
        this.ffns = new kubernetes.Namespace({
            name: "fission-function",
            labels: { name: "fission-function" },
        });
        this.dep = new kubernetes.Deployment("
    }
}

